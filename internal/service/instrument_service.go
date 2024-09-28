// Package service contains the service layer for the Moneybots API
package service

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/repository"
	"github.com/nsvirk/moneybotsapi/pkg/utils/state"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"gorm.io/gorm"
)

// InstrumentService is the service for managing instruments
type InstrumentService struct {
	repo  *repository.InstrumentRepository
	state *state.State
}

// NewInstrumentService creates a new instrument service
func NewInstrumentService(db *gorm.DB) *InstrumentService {
	stateManager, err := state.NewState(db)
	if err != nil {
		zaplogger.Fatal("failed to create state manager", zaplogger.Fields{"error": err})
	}
	return &InstrumentService{
		repo:  repository.NewInstrumentRepository(db),
		state: stateManager,
	}
}

// UpdateInstruments updates the instruments in the database
func (s *InstrumentService) UpdateInstruments() (int64, error) {
	// check if update is required
	lastUpdatedAt, err := s.state.Get("instruments_updated_at")
	if err == nil {
		if !s.isUpdateInstrumentsRequired(lastUpdatedAt) {
			zaplogger.Info("Instruments update not required", zaplogger.Fields{
				"lastUpdatedAt": lastUpdatedAt,
			})
			return 0, nil
		}
	}

	zaplogger.Info("Instruments update required", zaplogger.Fields{
		"lastUpdatedAt": lastUpdatedAt,
	})

	// get instruments from kite
	resp, err := http.Get("https://api.kite.trade/instruments")
	if err != nil {
		return 0, fmt.Errorf("failed to fetch instruments: %v", err)
	}
	defer resp.Body.Close()

	// parse response body to csv
	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("failed to parse CSV: %v", err)
	}

	records = records[1:] // Skip header row

	// truncate instruments table
	if err := s.repo.TruncateInstrumentsTable(); err != nil {
		return 0, fmt.Errorf("failed to truncate table: %v", err)
	}

	// insert instruments in batches
	batchSize := 500
	var totalInserted int64 = 0
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}

		// insert instruments in batch
		inserted, err := s.repo.InsertInstruments(records[i:end])

		if err != nil {
			return totalInserted, fmt.Errorf("failed to insert batch starting at index %d: %v", i, err)
		}
		totalInserted += inserted
	}

	// update state after all instruments have been updated
	if err := s.state.Set("instruments_updated_at", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		return 0, fmt.Errorf("failed to update state: %v", err)
	}

	zaplogger.Info("Instruments updated", zaplogger.Fields{
		"totalInserted": totalInserted,
	})

	// get instruments record count
	recordCount, err := s.repo.GetInstrumentsRecordCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get instruments record count: %v", err)
	}

	return recordCount, nil
}

// isUpdateInstrumentsRequired checks if the instruments need to be updated
func (s *InstrumentService) isUpdateInstrumentsRequired(lastUpdatedAt string) bool {

	// parse last updated at time
	lastUpdatedAtTime, err := time.Parse("2006-01-02 15:04:05", lastUpdatedAt)
	if err != nil {
		return true // If we can't parse the time, assume update is needed
	}

	// false only if last update is today and after 08:15am
	if lastUpdatedAtTime.Day() == time.Now().Day() {
		if lastUpdatedAtTime.Hour() == 8 && lastUpdatedAtTime.Minute() >= 15 {
			return false
		}

		if lastUpdatedAtTime.Hour() > 8 {
			return false
		}
	}

	return true
}

// GetTokensToInstrumentMap returns a map of token to instrument
func (s *InstrumentService) GetTokensToInstrumentMap(tokens []uint32) (map[string]string, error) {

	instruments, err := s.repo.GetInstrumentsByTokens(tokens)
	if err != nil {
		return nil, err
	}

	tokenToInstrumentMap := make(map[string]string)
	for _, instrument := range instruments {
		tokenStr := strconv.FormatUint(uint64(instrument.InstrumentToken), 10)
		tokenToInstrumentMap[tokenStr] = instrument.Exchange + ":" + instrument.Tradingsymbol
	}

	return tokenToInstrumentMap, nil
}

// GetInstrumentToTokenMap returns a map of instrument to token
func (s *InstrumentService) GetInstrumentToTokenMap(instruments []string) (map[string]uint32, error) {

	instrumentToTokenMap := make(map[string]uint32)
	for _, symbol := range instruments {
		parts := strings.Split(strings.TrimSpace(symbol), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid instrument format: %s", symbol)
		}

		exchange := strings.TrimSpace(parts[0])
		tradingsymbol := strings.TrimSpace(parts[1])

		instrument, err := s.repo.GetInstrumentByExchangeTradingsymbol(exchange, tradingsymbol)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Skip instruments that are not found
				continue
			}
			return nil, err
		}
		instrumentToTokenMap[symbol] = instrument.InstrumentToken
	}

	return instrumentToTokenMap, nil
}

// QueryInstruments queries the instruments table
func (s *InstrumentService) QueryInstruments(queryInstrumentsParams models.QueryInstrumentsParams, details string) ([]interface{}, error) {

	instruments, err := s.repo.QueryInstruments(queryInstrumentsParams)
	if err != nil {
		return nil, err
	}

	// make result as per details value
	result := make([]interface{}, len(instruments))
	if details == "t" {
		for i, instrument := range instruments {
			result[i] = fmt.Sprintf("%d", instrument.InstrumentToken)
		}
	} else if details == "i" {
		for i, instrument := range instruments {
			result[i] = fmt.Sprintf("%s:%s", instrument.Exchange, instrument.Tradingsymbol)
		}
	} else if details == "it" {
		for i, instrument := range instruments {
			result[i] = fmt.Sprintf("%s:%s:%d", instrument.Exchange, instrument.Tradingsymbol, instrument.InstrumentToken)
		}
	} else {
		for i, instrument := range instruments {
			result[i] = instrument
		}
	}

	return result, nil
}

// GetInstrumentsByExchange queries the instruments table by exchange and returns a list of instruments
func (s *InstrumentService) GetInstrumentsByExchange(exchange string) ([]models.InstrumentModel, error) {
	return s.repo.GetInstrumentsByExchange(exchange)
}

// GetInstrumentsByTradingsymbol queries the instruments table by tradingsymbol and returns a list of instruments
func (s *InstrumentService) GetInstrumentsByTradingsymbol(tradingsymbol string) ([]models.InstrumentModel, error) {
	return s.repo.GetInstrumentsByTradingsymbol(tradingsymbol)
}

// GetInstrumentsByInstrumentToken queries the instruments table by instrument token and returns a list of instruments
func (s *InstrumentService) GetInstrumentsByInstrumentToken(instrumentToken string) ([]models.InstrumentModel, error) {
	return s.repo.GetInstrumentsByInstrumentToken(instrumentToken)
}

// GetInstrumentsByExpiry queries the instruments table by expiry and returns a list of instruments
func (s *InstrumentService) GetInstrumentsByExpiry(expiry string) ([]models.InstrumentModel, error) {
	return s.repo.GetInstrumentsByExpiry(expiry)
}

// GetExchangeNamesByExpiry queries the instruments table by expiry and returns a list of distinct exchange, names
func (s *InstrumentService) GetExchangeNamesByExpiry(expiry string) ([]models.InstrumentModel, error) {
	return s.repo.GetExchangeNamesByExpiry(expiry)
}
