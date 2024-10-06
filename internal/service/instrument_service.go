// Package service contains the service layer for the Moneybots API
package service

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/repository"
	"github.com/nsvirk/moneybotsapi/pkg/utils/state"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"gorm.io/gorm"
)

var instrumentsUpdatedAtKey = "INSTRUMENTS_UPDATED_AT"

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
	instrumentsUpdatedAtValue, err := s.state.Get(instrumentsUpdatedAtKey)
	if err == nil {
		if !s.isUpdateInstrumentsRequired(instrumentsUpdatedAtValue) {
			zaplogger.Info("Instruments update not required", zaplogger.Fields{
				instrumentsUpdatedAtKey: instrumentsUpdatedAtValue,
			})
			return 0, nil
		}
	}

	zaplogger.Info("Instruments update required", zaplogger.Fields{
		instrumentsUpdatedAtKey: instrumentsUpdatedAtValue,
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
	if err := s.state.Set(instrumentsUpdatedAtKey, time.Now().Format("2006-01-02 15:04:05")); err != nil {
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

// GetInstrumentsInfoBySymbols returns instruments info for symbols
func (s *InstrumentService) GetInstrumentsInfoBySymbols(symbols []string) ([]models.InstrumentModel, error) {
	instrumentsResponse := make([]models.InstrumentModel, 0, len(symbols))
	for _, symbol := range symbols {
		parts := strings.Split(strings.TrimSpace(symbol), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid instrument format: %s", symbols)
		}
		exchange := strings.TrimSpace(parts[0])
		tradingsymbol := strings.TrimSpace(parts[1])

		instrument, err := s.repo.GetInstrumentByExchangeTradingsymbol(exchange, tradingsymbol)
		if err != nil {
			// Skip instruments that are not found
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		instrumentsResponse = append(instrumentsResponse, instrument)
	}
	return instrumentsResponse, nil
}

// GetInstrumentsInfoByTokens returns instruments info for tokens
func (s *InstrumentService) GetInstrumentsInfoByTokens(tokens []uint32) ([]models.InstrumentModel, error) {
	return s.repo.GetInstrumentsByTokens(tokens)
}

// QueryInstruments queries the instruments table
func (s *InstrumentService) GetInstrumentsQuery(queryInstrumentsParams models.QueryInstrumentsParams) ([]models.InstrumentModel, error) {
	return s.repo.GetInstrumentsQuery(queryInstrumentsParams)
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

// GetFNOSegmentWiseName returns a list of segment wise name for a given expiry
func (s *InstrumentService) GetFNOSegmentWiseName(expiry string) ([]models.InstrumentModel, error) {
	return s.repo.GetFNOSegmentWiseName(expiry)
}

// GetFNOSegmentWiseExpiry returns a list of segment wise expiry for a given name
func (s *InstrumentService) GetFNOSegmentWiseExpiry(name string, limit, offset int) ([]models.InstrumentModel, error) {
	return s.repo.GetFNOSegmentWiseExpiry(name, limit, offset)
}

// GetFNOOptionChain returns the option chain for a given instrument
func (s *InstrumentService) GetFNOOptionChain(exchange, name, futExpiry, optExpiry string) ([]models.InstrumentModel, error) {
	return s.repo.GetFNOOptionChain(exchange, name, futExpiry, optExpiry)
}
