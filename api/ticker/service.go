package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	kiteticker "github.com/nsvirk/gokiteticker"
	"github.com/redis/go-redis/v9"

	"gorm.io/gorm"
)

const (
	batchSize                       = 1000
	flushInterval                   = 100 * time.Microsecond
	channelCapacity                 = 100000
	channelCapacityWarningThreshold = 0.5 // 50% full
	monitorInterval                 = 10 * time.Second
)

type Service struct {
	repo        *Repository
	redisClient *redis.Client
	ticker      *kiteticker.Ticker
	mu          sync.Mutex
	isRunning   bool
	instruments map[uint32]string
	tickChannel chan kiteticker.Tick
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewService(db *gorm.DB, redisClient *redis.Client) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		repo:        NewRepository(db),
		redisClient: redisClient,
		isRunning:   false,
		instruments: make(map[uint32]string),
		tickChannel: make(chan kiteticker.Tick, channelCapacity),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (s *Service) Start(userID, enctoken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return fmt.Errorf("ticker is already running")
	}

	// Get all ticker instruments
	tickerInstruments, err := s.repo.GetTickerInstruments()
	if err != nil {
		return err
	}
	tickerInstrumentTokens := make([]uint32, len(tickerInstruments))
	for i, tickerInstrument := range tickerInstruments {
		tickerInstrumentTokens[i] = tickerInstrument.InstrumentToken
		s.instruments[tickerInstrument.InstrumentToken] = tickerInstrument.Instrument
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

	s.repo.Info("Ticker::Start", "Ticker started successfully")
	s.isRunning = true

	return nil
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return fmt.Errorf("ticker is not running")
	}

	s.ticker.Close()
	// wait one seconds for the ticker to close
	time.Sleep(1 * time.Second)
	s.ticker.Stop()
	s.ticker = nil
	s.isRunning = false
	s.cancel()

	s.repo.Info("Ticker::Stop", "Ticker stopped successfully")
	return nil
}

func (s *Service) Restart(userID, enctoken string) error {
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Start(userID, enctoken)
}

// Status returns the current status of the ticker
func (s *Service) Status() bool {
	return s.isRunning
}

func (s *Service) initializeTicker(userID, enctoken string) error {
	s.ticker = kiteticker.New(userID, enctoken)
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

func (s *Service) setupTickerCallbacks() {
	s.ticker.OnTick(func(tick kiteticker.Tick) {
		// fmt.Println(tick)
		s.tickChannel <- tick
	})

	s.ticker.OnConnect(func() {
		s.repo.Info("Ticker::OnConnect", "Connected to ticker")
		s.isRunning = true
	})

	s.ticker.OnError(func(err error) {
		s.repo.Error("Ticker::OnError", err.Error())
	})

	s.ticker.OnClose(func(code int, reason string) {
		s.repo.Warn("Ticker::OnClose", fmt.Sprintf("Closed with code %d: %s", code, reason))
		s.isRunning = false
	})

	s.ticker.OnReconnect(func(attempt int, delay time.Duration) {
		s.repo.Info("Ticker::OnReconnect", fmt.Sprintf("Reconnecting attempt %d with delay %v", attempt, delay))
	})

	s.ticker.OnNoReconnect(func(attempt int) {
		s.repo.Fatal("Ticker::OnNoReconnect", fmt.Sprintf("No reconnect after %d attempts", attempt))
	})
}

func (s *Service) processTicks() {
	var postgresData []TickerData
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

func (s *Service) processTick(tick kiteticker.Tick, postgresData *[]TickerData) {

	instrument, ok := s.instruments[tick.InstrumentToken]
	if !ok {
		s.repo.Error("Ticker::processTick", fmt.Sprintf("instrument not found for token %d", tick.InstrumentToken))
		return
	}

	// convert kiteticker.Tick to JSON
	// tickJson, err := json.Marshal(tick)
	// if err != nil {
	// 	s.repo.LogTickerEvent("Ticker::processTick", fmt.Sprintf("error marshaling tick to JSON: %v", tick.InstrumentToken))
	// }

	tickOHLCJson, err := json.Marshal(tick.OHLC)
	if err != nil {
		s.repo.Error("Ticker::processTick", fmt.Sprintf("error marshaling tick OHLC to JSON: %v", tick.InstrumentToken))

	}
	tickDepthJson, err := json.Marshal(tick.Depth)
	if err != nil {
		s.repo.Error("Ticker::processTick", fmt.Sprintf("error marshaling tick Depth to JSON: %v", tick.InstrumentToken))
	}

	// Round NetChange to 2 decimal points
	roundedNetChange := math.Round(tick.NetChange*100) / 100

	// convert kiteticker.Tick type to ticker.TickerData tyep
	tickerData := TickerData{
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

	// ToDo: Remove this print statement
	// fmt.Println("Processing tick for instrument", instrument)

}

func (s *Service) flushData(postgresData *[]TickerData) {

	if len(*postgresData) > 0 {
		if err := s.repo.UpsertTickerData(*postgresData); err != nil {
			s.repo.Error("Ticker::flushData:PostgresError", fmt.Sprintf("Failed to save ticks to Postgres: %v", err))
		}
		*postgresData = (*postgresData)[:0]
	}
}

func (s *Service) flushTicks() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.flushData(&[]TickerData{})
		}
	}
}

func (s *Service) TruncateTickerData() error {
	return s.repo.TruncateTickerData()
}

func (s *Service) AddTickerInstruments(instruments []string) (map[string]interface{}, error) {
	instrumentTokens, notFoundInstruments, err := s.getInstrumentTokens(instruments)
	if err != nil {
		return nil, err
	}

	if len(instrumentTokens) == 0 {
		return nil, fmt.Errorf("no valid instruments found")
	}

	tickerInstruments := make([]TickerInstrument, 0, len(instrumentTokens))
	for instrument, token := range instrumentTokens {
		tickerInstruments = append(tickerInstruments, TickerInstrument{
			Instrument:      instrument,
			InstrumentToken: token,
			UpdatedAt:       time.Now(),
		})
	}

	addedCount, updatedCount, err := s.repo.UpsertTickerInstruments(tickerInstruments)
	if err != nil {
		return nil, err
	}

	totalCount, err := s.repo.GetTickerInstrumentCount()
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"added":    addedCount,
		"existing": updatedCount,
		"total":    totalCount,
	}

	if len(notFoundInstruments) > 0 {
		response["missing"] = notFoundInstruments
	}

	return response, nil
}

func (s *Service) DeleteTickerInstruments(instruments []string) (int64, error) {
	return s.repo.DeleteTickerInstruments(instruments)
}

func (s *Service) GetTickerInstruments() ([]TickerInstrument, error) {
	return s.repo.GetTickerInstruments()
}

func (s *Service) GetTickerInstrumentCount() (int64, error) {
	return s.repo.GetTickerInstrumentCount()
}

func (s *Service) getInstrumentTokens(instruments []string) (map[string]uint32, []string, error) {
	instrumentTokens := make(map[string]uint32)
	var notFoundInstruments []string

	for _, instrument := range instruments {
		parts := strings.Split(instrument, ":")
		if len(parts) != 2 {
			notFoundInstruments = append(notFoundInstruments, instrument)
			continue
		}
		exchange, symbol := parts[0], parts[1]
		token, err := s.repo.GetInstrumentToken(exchange, symbol)
		if err != nil {
			notFoundInstruments = append(notFoundInstruments, instrument)
		} else {
			instrumentTokens[instrument] = token
		}
	}

	return instrumentTokens, notFoundInstruments, nil
}

func (s *Service) TruncateTickerInstruments() error {
	return s.repo.TruncateTickerInstruments()
}

func (s *Service) UpsertTickerInstruments(instruments []TickerInstrument) (int, int, error) {
	return s.repo.UpsertTickerInstruments(instruments)
}

// UpsertQueriedInstruments
func (s *Service) UpsertQueriedInstruments(exchange, tradingsymbol, expiry, strike string) (map[string]interface{}, error) {
	return s.repo.UpsertQueriedInstruments(exchange, tradingsymbol, expiry, strike)
}

func (s *Service) GetNFOFilterMonths() (string, string, string) {
	now := time.Now()
	month0 := strings.ToUpper(now.Format("06Jan"))
	month1 := strings.ToUpper(now.AddDate(0, 1, 0).Format("06Jan"))
	month2 := strings.ToUpper(now.AddDate(0, 2, 0).Format("06Jan"))
	return month0, month1, month2
}

func (s *Service) monitorTickerChannel() {
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
				s.repo.Warn("Ticker::ChannelWarning", warningMsg)

				// You might want to take additional actions here, such as:
				// - Slowing down the ticker
				// - Increasing processing speed
				// - Alerting operations team
			}
			// ToDo: Remove this print statement
			// warningMsg := fmt.Sprintf("Ticker channel is %.2f%% full (%d/%d)", capacityPercentage*100, currentCapacity, channelCapacity)
			// fmt.Println(warningMsg)
		}
	}
}
