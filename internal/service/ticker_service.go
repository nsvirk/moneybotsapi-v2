// Package service contains the service layer for the Moneybots API
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	kiteticker "github.com/nsvirk/gokiteticker"
	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/repository"
	"github.com/redis/go-redis/v9"

	"gorm.io/gorm"
)

const tickerReconnectMaxRetries = 10 // 10 retries

// TickerService
const (
	batchSize                       = 1000
	flushInterval                   = 100 * time.Microsecond
	channelCapacity                 = 100000
	channelCapacityWarningThreshold = 0.5 // 50% full
	monitorInterval                 = 10 * time.Second
)

type UpsertQueriedInstrumentsResult struct {
	Queried  int64
	Inserted int64
	Updated  int64
	Total    int64
}

type TickerService struct {
	repo              *repository.TickerRepository
	redisClient       *redis.Client
	ticker            *kiteticker.Ticker
	mu                sync.Mutex
	isRunning         bool
	instruments       map[uint32]string
	tickChannel       chan kiteticker.Tick
	ctx               context.Context
	cancel            context.CancelFunc
	instrumentService *InstrumentService
	indexService      *IndexService
}

// NewService creates a new TickerService
func NewTickerService(db *gorm.DB, redisClient *redis.Client) *TickerService {
	ctx, cancel := context.WithCancel(context.Background())
	return &TickerService{
		repo:              repository.NewTickerRepository(db),
		redisClient:       redisClient,
		isRunning:         false,
		instruments:       make(map[uint32]string),
		tickChannel:       make(chan kiteticker.Tick, channelCapacity),
		ctx:               ctx,
		cancel:            cancel,
		instrumentService: NewInstrumentService(db),
		indexService:      NewIndexService(db),
	}
}

// Start starts the ticker service
func (s *TickerService) Start(userID, enctoken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop the ticker if already runnin
	if s.isRunning {
		s.Stop(userID)
		time.Sleep(2 * time.Second)
	}

	// Get all ticker instruments
	tickerInstruments, err := s.repo.GetTickerInstruments(userID)
	if err != nil {
		return err
	}
	tickerInstrumentTokens := make([]uint32, len(tickerInstruments))
	for i, tickerInstrument := range tickerInstruments {
		instrumentToken := tickerInstrument.InstrumentToken
		instrument := tickerInstrument.Instrument
		tickerInstrumentTokens[i] = instrumentToken
		s.instruments[instrumentToken] = instrument
	}

	if len(tickerInstrumentTokens) == 0 {
		return fmt.Errorf("no instruments to subscribe")
	}

	// Initialize ticker
	if err := s.initializeTicker(userID, enctoken); err != nil {
		return err
	}

	// Subscribe to instruments
	if err := s.ticker.Subscribe(tickerInstrumentTokens); err != nil {
		return err
	}

	// Set ticker mode to full
	if err := s.ticker.SetMode(kiteticker.ModeFull, tickerInstrumentTokens); err != nil {
		return err
	}

	go s.processTicks()
	go s.flushTicks()
	go s.monitorTickerChannel()

	s.repo.Info("Start", "Ticker started successfully")
	s.isRunning = true

	return nil
}

// Stop stops the ticker service
func (s *TickerService) Stop(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return fmt.Errorf("ticker is not running")
	}

	// Get all ticker instruments
	tickerInstruments, err := s.repo.GetTickerInstruments(userID)
	if err != nil {
		return err
	}
	tickerInstrumentTokens := make([]uint32, len(tickerInstruments))
	for i, tickerInstrument := range tickerInstruments {
		instrumentToken := tickerInstrument.InstrumentToken
		instrument := tickerInstrument.Instrument
		tickerInstrumentTokens[i] = instrumentToken
		s.instruments[instrumentToken] = instrument
	}

	// Unsubscribe from instruments
	s.ticker.Unsubscribe(tickerInstrumentTokens)
	time.Sleep(1 * time.Second)

	// Stop the ticker
	s.ticker.Close()
	s.ticker.Stop()
	s.ticker = nil
	s.isRunning = false

	// s.cancel() // if this is enable then the ticker doesnt run on next start

	s.repo.Info("Stop", "Ticker stopped successfully")
	return nil
}

func (s *TickerService) Restart(userID, enctoken string) error {
	return s.Start(userID, enctoken)
}

// Status returns the current status of the ticker
func (s *TickerService) Status() bool {
	return s.isRunning
}

// initializeTicker initializes the ticker
func (s *TickerService) initializeTicker(userID, enctoken string) error {
	s.ticker = kiteticker.New(userID, enctoken)

	s.SetReconnectMaxRetries(tickerReconnectMaxRetries)
	s.setupTickerCallbacks()

	go s.ticker.Serve()

	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.isRunning {
				return nil
			}
		case <-timeout:
			return fmt.Errorf("timeout waiting for ticker connection")
		}
	}
}
func (s *TickerService) SetReconnectMaxRetries(retries int) {
	s.ticker.SetReconnectMaxRetries(retries)
}

// setupTickerCallbacks sets up the ticker callbacks
func (s *TickerService) setupTickerCallbacks() {
	s.ticker.OnTick(func(tick kiteticker.Tick) {
		// fmt.Println(tick)
		s.tickChannel <- tick
	})

	s.ticker.OnConnect(func() {
		s.repo.Info("OnConnect", "Connected to ticker")
		s.isRunning = true
	})

	s.ticker.OnError(func(err error) {
		s.repo.Error("OnError", err.Error())
	})

	s.ticker.OnClose(func(code int, reason string) {
		s.repo.Warn("OnClose", fmt.Sprintf("Closed with code %d: %s", code, reason))
		s.isRunning = false
	})

	s.ticker.OnReconnect(func(attempt int, delay time.Duration) {
		s.repo.Info("OnReconnect", fmt.Sprintf("Reconnecting attempt %d with delay %v", attempt, delay))
	})

	s.ticker.OnNoReconnect(func(attempt int) {
		s.repo.Fatal("OnNoReconnect", fmt.Sprintf("No reconnect after %d attempts", attempt))
	})
}

func (s *TickerService) processTicks() {
	var postgresData []models.TickerData
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case tick := <-s.tickChannel:
			s.processTick(tick, &postgresData)
		case <-ticker.C:
			s.flushData(&postgresData)
		}

		if len(postgresData) >= batchSize {
			s.flushData(&postgresData)
		}
	}
}

// processTick processes the tick
func (s *TickerService) processTick(tick kiteticker.Tick, postgresData *[]models.TickerData) {

	instrument, ok := s.instruments[tick.InstrumentToken]
	if !ok {
		s.repo.Error("processTick", fmt.Sprintf("instrument not found for token %d", tick.InstrumentToken))
		return
	}

	// convert kiteticker.Tick to JSON
	// tickJson, err := json.Marshal(tick)
	// if err != nil {
	// 	s.repo.LogTickerEvent("processTick", fmt.Sprintf("error marshaling tick to JSON: %v", tick.InstrumentToken))
	// }

	tickOHLCJson, err := json.Marshal(tick.OHLC)
	if err != nil {
		s.repo.Error("processTick", fmt.Sprintf("error marshaling tick OHLC to JSON: %v", tick.InstrumentToken))

	}
	tickDepthJson, err := json.Marshal(tick.Depth)
	if err != nil {
		s.repo.Error("processTick", fmt.Sprintf("error marshaling tick Depth to JSON: %v", tick.InstrumentToken))
	}

	// Round NetChange to 2 decimal points
	roundedNetChange := math.Round(tick.NetChange*100) / 100

	// convert kiteticker.Tick type to ticker.TickerData tyep
	tickerData := models.TickerData{
		// custom
		Instrument: instrument,
		// from kiteticker.Tick
		Mode:               tick.Mode,
		InstrumentToken:    tick.InstrumentToken,
		IsTradable:         tick.IsTradable,
		IsIndex:            tick.IsIndex,
		Timestamp:          tick.Timestamp.Time,
		LastTradeTime:      tick.LastTradeTime.Time,
		LastPrice:          tick.LastPrice,
		LastTradedQuantity: tick.LastTradedQuantity,
		TotalBuyQuantity:   tick.TotalBuyQuantity,
		TotalSellQuantity:  tick.TotalSellQuantity,
		VolumeTraded:       tick.VolumeTraded,
		// TotalBuy:           tick.TotalBuy,
		// TotalSell:          tick.TotalSell,
		AverageTradePrice: tick.AverageTradePrice,
		OI:                tick.OI,
		OIDayHigh:         tick.OIDayHigh,
		OIDayLow:          tick.OIDayLow,
		NetChange:         roundedNetChange,
		OHLC:              tickOHLCJson,
		Depth:             tickDepthJson,
		// Tick:               tickJson,
		UpdatedAt: time.Now(),
	}

	// ---- SAVE TO POSTGRES -----------------------------------------
	// Append the tick to the Postgres data slice
	*postgresData = append(*postgresData, tickerData)
}

// flushData flushes the data to postgres
func (s *TickerService) flushData(postgresData *[]models.TickerData) {

	if len(*postgresData) > 0 {
		if err := s.repo.UpsertTickerData(*postgresData); err != nil {
			s.repo.Error("flushData", fmt.Sprintf("Failed to save ticks to Postgres: %v", err))
		}
		*postgresData = (*postgresData)[:0]
	}
}

// flushTicks flushes the ticks to postgres
func (s *TickerService) flushTicks() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.flushData(&[]models.TickerData{})
		}
	}
}

// TruncateTickerData truncates the ticker data
func (s *TickerService) TruncateTickerData() error {
	return s.repo.TruncateTickerData()
}

// AddTickerInstruments adds the ticker instruments
func (s *TickerService) AddTickerInstruments(userID string, instruments []string) (map[string]interface{}, error) {

	// make instrumentsTokensMap using instrument service
	instrumentsTokensMap, err := s.instrumentService.GetInstrumentToTokenMap(instruments)
	if err != nil {
		return nil, err
	}

	// find missing instruments
	missingInstruments := make([]string, 0)
	for _, instrument := range instruments {
		if _, ok := instrumentsTokensMap[instrument]; !ok {
			missingInstruments = append(missingInstruments, instrument)
		}
	}

	// upsert the instruments
	insertedCount, updatedCount, err := s.repo.UpsertTickerInstruments(userID, instrumentsTokensMap)
	if err != nil {
		return nil, err
	}

	totalCount, err := s.repo.GetTickerInstrumentCount(userID)
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"inserted": insertedCount,
		"updated":  updatedCount,
		"missing":  len(missingInstruments),
		"total":    totalCount,
	}

	if len(missingInstruments) > 0 {
		response["missing_instruments"] = missingInstruments
	}

	return response, nil
}

func (s *TickerService) DeleteTickerInstruments(userID string, instruments []string) (int64, error) {
	return s.repo.DeleteTickerInstruments(userID, instruments)
}

// GetTickerInstruments gets the ticker instruments
func (s *TickerService) GetTickerInstruments(userID string) ([]models.TickerInstrument, error) {
	return s.repo.GetTickerInstruments(userID)
}

// GetTickerInstrumentCount gets the ticker instrument count
func (s *TickerService) GetTickerInstrumentCount(userID string) (int64, error) {
	return s.repo.GetTickerInstrumentCount(userID)
}

// TruncateTickerInstruments truncates the ticker instruments
func (s *TickerService) TruncateTickerInstruments() (int64, error) {
	return s.repo.TruncateTickerInstruments()
}

// UpsertQueriedInstruments upserts the queried instruments
func (s *TickerService) UpsertQueriedInstruments(userID, exchange, tradingsymbol, name, expiry, strike, segment, instrumentType string) (UpsertQueriedInstrumentsResult, error) {

	var result UpsertQueriedInstrumentsResult
	// query instrumetns using instruments service
	queryInstrumentsParams := models.QueryInstrumentsParams{
		Exchange:       exchange,
		Tradingsymbol:  tradingsymbol,
		Name:           name,
		Expiry:         expiry,
		Strike:         strike,
		Segment:        segment,
		InstrumentType: instrumentType,
	}
	details := "it"
	queriedInstruments, err := s.instrumentService.QueryInstruments(queryInstrumentsParams, details)
	if err != nil {
		return result, err
	}

	// convert queried instruments to []tokens
	instrumentsTokenMap := make(map[string]uint32, len(queriedInstruments))
	for _, instrument := range queriedInstruments {
		// exchange:tradingsymbol:instrument_token
		instrumentDetails := strings.Split(instrument.(string), ":")
		exchange := instrumentDetails[0]
		tradingsymbol := instrumentDetails[1]
		instrumentTokenStr := instrumentDetails[2]
		instrumentToken, err := strconv.ParseUint(instrumentTokenStr, 10, 32)
		if err != nil {
			// handle error
		}
		instrumentsTokenMap[exchange+":"+tradingsymbol] = uint32(instrumentToken)
	}

	// upsert the queried instruments
	insertedCount, updatedCount, err := s.repo.UpsertTickerInstruments(userID, instrumentsTokenMap)
	if err != nil {
		return result, err
	}

	result = UpsertQueriedInstrumentsResult{
		Queried:  int64(len(queriedInstruments)),
		Inserted: insertedCount,
		Updated:  updatedCount,
		Total:    insertedCount + updatedCount,
	}

	return result, nil
}

// monitorTickerChannel monitors the ticker channel
func (s *TickerService) monitorTickerChannel() {
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			currentCapacity := len(s.tickChannel)
			capacityPercentage := float64(currentCapacity) / float64(channelCapacity)

			if capacityPercentage >= channelCapacityWarningThreshold {
				warningMsg := fmt.Sprintf("Ticker channel is %.2f%% full (%d/%d)", capacityPercentage*100, currentCapacity, channelCapacity)
				s.repo.Warn("ChannelWarning", warningMsg)

				// You might want to take additional actions here, such as:
				// - Alerting operations team
			}
		}
	}
}
