// Package service contains the service layer for the Moneybots API
package service

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/repository"
	"github.com/nsvirk/moneybotsapi/pkg/utils/state"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"gorm.io/gorm"
)

// var NSEIndicesBaseURL = "https://nsearchives.nseindia.com/content/indices/"
// var NSEIndicesBaseURL = "https://niftyindices.com/IndexConstituent/"
var NSEIndicesBaseURL = "https://raw.githubusercontent.com/nsvirk/nseindicesdata/refs/heads/main/csvfiles/"

var NSEIndicesFileMap = map[string]string{
	"NSE:NIFTY 50":                 "ind_nifty50list.csv",
	"NSE:NIFTY NEXT 50":            "ind_niftynext50list.csv",
	"NSE:NIFTY 100":                "ind_nifty100list.csv",
	"NSE:NIFTY 200":                "ind_nifty200list.csv",
	"NSE:NIFTY TOTAL MARKET":       "ind_niftytotalmarket_list.csv",
	"NSE:NIFTY 500":                "ind_nifty500list.csv",
	"NSE:NIFTY MIDCAP 50":          "ind_niftymidcap50list.csv",
	"NSE:NIFTY MIDCAP 100":         "ind_niftymidcap100list.csv",
	"NSE:NIFTY SMALLCAP 100":       "ind_niftysmallcap100list.csv",
	"NSE:NIFTY AUTO":               "ind_niftyautolist.csv",
	"NSE:NIFTY BANK":               "ind_niftybanklist.csv",
	"NSE:NIFTY FINANCIAL SERVICES": "ind_niftyfinancelist.csv",
	"NSE:NIFTY HEALTHCARE":         "ind_niftyhealthcarelist.csv",
	"NSE:NIFTY IT":                 "ind_niftyitlist.csv",
	"NSE:NIFTY FMCG":               "ind_niftyfmcglist.csv",
	"NSE:NIFTY METAL":              "ind_niftymetallist.csv",
	"NSE:NIFTY PHARMA":             "ind_niftypharmalist.csv",
	"NSE:NIFTY REALTY":             "ind_niftyrealtylist.csv",
	"NSE:NIFTY CONSUMER DURABLES":  "ind_niftyconsumerdurableslist.csv",
	"NSE:NIFTY OIL GAS":            "ind_niftyoilgaslist.csv",
}

// IndexService is the service for managing indices
type IndexService struct {
	client         *http.Client
	repo           *repository.IndexRepository
	instrumentRepo *repository.InstrumentRepository
	state          *state.State
}

// NewIndexService creates a new IndexService
func NewIndexService(db *gorm.DB) *IndexService {
	stateManager, err := state.NewState(db)
	if err != nil {
		zaplogger.Fatal("failed to create state manager", zaplogger.Fields{"error": err})
	}

	return &IndexService{
		client:         &http.Client{},
		repo:           repository.NewIndexRepository(db),
		instrumentRepo: repository.NewInstrumentRepository(db),
		state:          stateManager,
	}
}

// UpdateNSEIndices fetches the instruments for a given NSE index and updates the database
func (s *IndexService) UpdateNSEIndices() (int64, error) {

	// check if update is required
	lastUpdatedAt, err := s.state.Get("indices_updated_at")
	if err == nil {
		if !s.isUpdateIndicesRequired(lastUpdatedAt) {
			zaplogger.Info("Indices update not required", zaplogger.Fields{
				"lastUpdatedAt": lastUpdatedAt,
			})
			return 0, nil
		}
	}

	// update log with logger
	zaplogger.Info("Indices update required", zaplogger.Fields{
		"lastUpdatedAt": lastUpdatedAt,
	})

	// truncate table
	if err := s.repo.TruncateIndicesTable(); err != nil {
		return 0, fmt.Errorf("failed to truncate table: %v", err)
	}

	// get instruments for all indices
	var totalInserted int64
	indices, err := s.GetNSEIndexNamesFromNSEIndicesFileMap()
	if err != nil {
		return 0, fmt.Errorf("failed to get indices: %v", err)
	}

	// update indices
	for _, index := range indices {
		// get records for index
		indexRecords, err := s.FetchNSEIndexInstruments(index)
		if err != nil {
			return 0, fmt.Errorf("failed to get instruments for index %s: %v", index, err)
		}

		count, err := s.repo.InsertIndices(indexRecords)
		if err != nil {
			return 0, fmt.Errorf("failed to create instruments for index %s: %v", index, err)
		}
		totalInserted += count

	}

	// update state after all indices have been updated
	if err := s.state.Set("indices_updated_at", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		return 0, fmt.Errorf("failed to update state: %v", err)
	}

	zaplogger.Info("Indices updated", zaplogger.Fields{
		"totalInserted": totalInserted,
	})

	// get indices record count
	recordCount, err := s.repo.GetIndicesRecordCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get indices record count: %v", err)
	}

	return recordCount, nil

}

// isUpdateIndicesRequired checks if the indices need to be updated
// if last update time is not today, return true
func (s *IndexService) isUpdateIndicesRequired(lastUpdatedAt string) bool {

	// parse last updated at time
	lastUpdatedAtTime, err := time.Parse("2006-01-02 15:04:05", lastUpdatedAt)
	if err != nil {
		return true // If we can't parse the time, assume update is needed
	}

	// check if last update date is today return false
	if lastUpdatedAtTime.Day() == time.Now().Day() {
		return false
	}

	return true
}

// FetchNSEIndexInstruments fetches the instruments for a given NSE index
func (s *IndexService) FetchNSEIndexInstruments(indexName string) ([]models.IndexModel, error) {

	// -------------------------------------------------------------------------------------------------
	// make request to index url
	// -------------------------------------------------------------------------------------------------
	// get index csv file name
	indexCsvFile, ok := NSEIndicesFileMap[indexName]
	if !ok {
		return nil, fmt.Errorf("invalid index: %s", indexName)
	}

	// make url
	url := fmt.Sprintf("%s%s", NSEIndicesBaseURL, indexCsvFile)

	// create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for index %s: %v", indexName, err)
	}

	// set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Referer", "https://niftyindices.com/")

	// make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download CSV for index %s: %v", indexName, err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV for index %s: %v", indexName, err)
	}

	indexRecords := make([]models.IndexModel, 0, len(records)-1)
	for _, record := range records[1:] { // Skip header row
		// record : [Company Name, Industry, Symbol, Series, ISIN Code]
		if len(record) < 5 {
			continue
		}
		indexRecords = append(indexRecords, models.IndexModel{
			Index:         indexName,
			Exchange:      "NSE",
			CompanyName:   record[0],
			Industry:      record[1],
			Tradingsymbol: record[2],
			Series:        record[3],
			ISINCode:      record[4],
		})
	}

	return indexRecords, nil
}

// GetNSEIndexNamesFromFileMap fetches the names of all NSE indices from NSEIndicesFileMap
func (s *IndexService) GetNSEIndexNamesFromNSEIndicesFileMap() ([]string, error) {
	var indices []string
	for index := range NSEIndicesFileMap {
		indices = append(indices, index)
	}
	return indices, nil
}

// GetNSEIndexNames returns the names of all NSE indices
func (s *IndexService) GetNSEIndexNames() ([]string, error) {
	return s.repo.GetNSEIndexNames()
}

// GetNSEIndexInstruments fetches the instruments for a given NSE index
func (s *IndexService) GetNSEIndexInstruments(indexName, details string) ([]interface{}, error) {
	indexRecords, err := s.repo.GetNSEIndexInstruments(indexName)
	if err != nil {
		return nil, err
	}

	// get instruments from index repo
	var exchange string
	indexTradingsymbols := make([]string, len(indexRecords))
	for i, indexRecord := range indexRecords {
		exchange = indexRecord.Exchange
		indexTradingsymbols[i] = indexRecord.Tradingsymbol
	}

	// get instruments from instrument repo
	instruments, err := s.instrumentRepo.GetInstrumentByExchangeTradingsymbols(exchange, indexTradingsymbols)
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
