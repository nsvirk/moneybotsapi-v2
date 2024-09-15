// File: github.com/nsvirk/moneybotsapi/instrument/service.go

package instrument

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/nsvirk/moneybotsapi/shared/zaplogger"
	"gorm.io/gorm"
)

type InstrumentService struct {
	repo *Repository
}

func NewInstrumentService(db *gorm.DB) *InstrumentService {
	return &InstrumentService{
		repo: NewInstrumentRepository(db),
	}
}

func (s *InstrumentService) UpdateInstruments() (int, error) {
	resp, err := http.Get("https://api.kite.trade/instruments")
	if err != nil {
		return 0, fmt.Errorf("failed to fetch instruments: %v", err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("failed to parse CSV: %v", err)
	}

	records = records[1:] // Skip header row

	if err := s.repo.TruncateInstruments(); err != nil {
		return 0, fmt.Errorf("failed to truncate table: %v", err)
	}

	batchSize := 500
	totalInserted := 0
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}

		inserted, err := s.repo.InsertInstruments(records[i:end])
		if err != nil {
			zaplogger.Error("Failed to insert batch", zaplogger.Fields{"startIndex": i, "error": err})
			return totalInserted, fmt.Errorf("failed to insert batch starting at index %d: %v", i, err)
		}
		totalInserted += inserted
	}

	return totalInserted, nil
}

func (s *InstrumentService) GetInstrumentSymbols(tokens []uint32) (map[string]string, error) {
	instruments, err := s.repo.GetInstrumentsByTokens(tokens)
	if err != nil {
		return nil, err
	}

	instrumentMap := make(map[string]string)
	for _, instrument := range instruments {
		tokenStr := strconv.FormatUint(uint64(instrument.InstrumentToken), 10)
		instrumentMap[tokenStr] = instrument.Exchange + ":" + instrument.Tradingsymbol
	}

	return instrumentMap, nil
}

func (s *InstrumentService) GetInstrumentToTokenMap(instruments []string) (map[string]uint32, error) {
	instrumentMap := make(map[string]uint32)
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
		instrumentMap[symbol] = instrument.InstrumentToken
	}

	return instrumentMap, nil
}

func (s *InstrumentService) QueryInstruments(exchange, tradingsymbol, expiry, strike, segment string) ([]InstrumentModel, error) {
	return s.repo.QueryInstruments(exchange, tradingsymbol, expiry, strike, segment)
}

// GetOptionChainNames returns a list of exchange:name for a given expiry
func (s *InstrumentService) GetOptionChainNames(expiry string) ([]string, error) {
	return s.repo.GetExchangeNamesForExpiry(expiry)
}

// GetOptionChainInstruments returns a list of instruments for a given exchange, name and expiry
func (s *InstrumentService) GetOptionChainInstruments(exchange, name, expiry string, tokensBool, detailsBool bool) ([]interface{}, error) {
	instruments, err := s.repo.GetOptionChainInstruments(exchange, name, expiry)
	if err != nil {
		return nil, err
	}

	if detailsBool {
		result := make([]interface{}, len(instruments))
		for i, v := range instruments {
			result[i] = v
		}
		return result, nil

	} else if tokensBool {
		tokensList := make([]interface{}, len(instruments))
		for i, instrument := range instruments {
			tokensList[i] = fmt.Sprintf("%d", instrument.InstrumentToken)
		}
		return tokensList, nil

	} else {
		symbolsList := make([]interface{}, len(instruments))
		for i, instrument := range instruments {
			symbolsList[i] = fmt.Sprintf("%s:%s", instrument.Exchange, instrument.Tradingsymbol)
		}
		return symbolsList, nil
	}
}
