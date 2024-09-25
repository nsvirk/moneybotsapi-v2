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
	"github.com/nsvirk/moneybotsapi/pkg/utils/logger"
	"github.com/nsvirk/moneybotsapi/pkg/utils/state"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"gorm.io/gorm"
)

// InstrumentService is the service for managing instruments
type InstrumentService struct {
	repo   *repository.InstrumentRepository
	state  *state.State
	logger *logger.Logger
}

// NewInstrumentService creates a new instrument service
func NewInstrumentService(db *gorm.DB) *InstrumentService {
	stateManager, err := state.NewState(db)
	if err != nil {
		zaplogger.Fatal("failed to create state manager", zaplogger.Fields{"error": err})
	}
	logger, err := logger.New(db, "INSTRUMENT SERVICE")
	if err != nil {
		zaplogger.Error("failed to create instrument logger", zaplogger.Fields{"error": err})
	}

	return &InstrumentService{
		repo:   repository.NewInstrumentRepository(db),
		state:  stateManager,
		logger: logger,
	}
}

// UpdateInstruments updates the instruments in the database
func (s *InstrumentService) UpdateInstruments() (int, error) {
	// check if update is required
	lastUpdatedAt, err := s.state.Get("instruments_updated_at")
	if err == nil {
		if !s.isUpdateInstrumentsRequired(lastUpdatedAt) {
			// update log with logger
			s.logger.Info("Instruments update not required", map[string]interface{}{
				"lastUpdatedAt": lastUpdatedAt,
			})
			return 0, nil
		}
	}
	// update log with logger
	s.logger.Info("Instruments update required", map[string]interface{}{
		"lastUpdatedAt": lastUpdatedAt,
	})

	// get instruments from kite
	resp, err := http.Get("https://api.kite.trade/instruments")
	if err != nil {
		s.logger.Error("Failed to fetch instruments", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to fetch instruments: %v", err)
	}
	defer resp.Body.Close()

	// parse response body to csv
	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		s.logger.Error("Failed to parse CSV", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to parse CSV: %v", err)
	}

	records = records[1:] // Skip header row

	// truncate instruments table
	if err := s.repo.TruncateInstruments(); err != nil {
		s.logger.Error("Failed to truncate table", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to truncate table: %v", err)
	}

	// insert instruments in batches
	batchSize := 500
	totalInserted := 0
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}

		// insert instruments in batch
		inserted, err := s.repo.InsertInstruments(records[i:end])

		if err != nil {
			s.logger.Error("Failed to insert batch", map[string]interface{}{
				"startIndex": i,
				"error":      err,
			})
			return totalInserted, fmt.Errorf("failed to insert batch starting at index %d: %v", i, err)
		}
		totalInserted += inserted
	}

	// update state after all instruments have been updated
	if err := s.state.Set("instruments_updated_at", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		s.logger.Error("Failed to update state", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to update state: %v", err)
	}

	// update log with logger
	s.logger.Info("Instruments updated", map[string]interface{}{
		"totalInserted": totalInserted,
	})

	// get instruments record count
	recordCount, err := s.repo.GetInstrumentsRecordCount()
	if err != nil {
		s.logger.Error("Failed to get instruments record count", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to get instruments record count: %v", err)
	}

	// insert record count in logs
	s.logger.Info("Instruments record count", map[string]interface{}{
		"recordCount": recordCount,
	})

	return totalInserted, nil
}

// isUpdateInstrumentsRequired checks if the instruments need to be updated
func (s *InstrumentService) isUpdateInstrumentsRequired(lastUpdatedAt string) bool {

	// parse last updated at time
	lastUpdatedAtTime, err := time.Parse("2006-01-02 15:04:05", lastUpdatedAt)
	if err != nil {
		return true // If we can't parse the time, assume update is needed
	}

	// check if last update date is today
	if lastUpdatedAtTime.Day() == time.Now().Day() {
		// if last update hour is before 08:00 return true
		if lastUpdatedAtTime.Hour() < 8 {
			return true
		}
		// if last update hour is 08:00 AM,
		if lastUpdatedAtTime.Hour() == 8 {
			// if last update minute is less than 05 return true
			if lastUpdatedAtTime.Minute() < 5 {
				return true
			}
			// if last update minute is 05 return false
			if lastUpdatedAtTime.Minute() == 5 {
				return false
			}
			// if last update minute is after 05 return false
			if lastUpdatedAtTime.Minute() > 5 {
				return false
			}
		}

		// if last update hour is after 08:00 AM, return false
		if lastUpdatedAtTime.Hour() > 8 {
			return false
		}
	}

	return false
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
func (s *InstrumentService) QueryInstruments(exchange, tradingsymbol, expiry, strike, segment string) ([]models.InstrumentModel, error) {
	return s.repo.QueryInstruments(exchange, tradingsymbol, expiry, strike, segment)
}

// GetOptionChainNames returns a list of exchange:name for a given expiry
func (s *InstrumentService) GetOptionChainNames(expiry string) ([]string, error) {
	return s.repo.GetOptionChainNames(expiry)
}

// GetOptionChainInstruments returns a list of instruments for a given exchange, name and expiry
func (s *InstrumentService) GetOptionChainInstruments(exchange, name, expiry, returnType string) ([]interface{}, error) {
	// get the return type
	var returnTokens, returnInstruments, returnInstrumentsWithTokens, returnAll bool
	if returnType == "tokens" {
		returnTokens = true
	} else if returnType == "instruments" {
		returnInstruments = true
	} else if returnType == "instruments_with_tokens" {
		returnInstrumentsWithTokens = true
	} else {
		returnAll = true
	}

	instruments, err := s.repo.GetOptionChainInstruments(exchange, name, expiry)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(instruments))
	for i, instrument := range instruments {
		if returnTokens {
			result[i] = fmt.Sprintf("%d", instrument.InstrumentToken)
			continue

		} else if returnInstruments {
			result[i] = fmt.Sprintf("%s:%s", instrument.Exchange, instrument.Tradingsymbol)
			continue

		} else if returnInstrumentsWithTokens {
			result[i] = fmt.Sprintf("%s:%s:%d", instrument.Exchange, instrument.Tradingsymbol, instrument.InstrumentToken)
			continue

		} else if returnAll {
			result[i] = instrument
			continue
		}
	}

	return result, nil

}
