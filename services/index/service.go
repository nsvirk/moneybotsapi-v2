// Package index manages the Index instruments
// service.go - Core service logic
package index

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/nsvirk/moneybotsapi/shared/logger"
	"github.com/nsvirk/moneybotsapi/shared/state"
	"github.com/nsvirk/moneybotsapi/shared/zaplogger"
	"gorm.io/gorm"
)

// NSEIndicesURLMap is a map of NSE indices and their corresponding URLs
var NSEIndicesURLMap = map[string]string{
	"NSE:NIFTY 50":      "https://archives.nseindia.com/content/indices/ind_nifty50list.csv",
	"NSE:NIFTY 100":     "https://archives.nseindia.com/content/indices/ind_nifty100list.csv",
	"NSE:NIFTY 200":     "https://archives.nseindia.com/content/indices/ind_nifty200list.csv",
	"NSE:NIFTY 500":     "https://archives.nseindia.com/content/indices/ind_nifty500list.csv",
	"NSE:NIFTY BANK":    "https://archives.nseindia.com/content/indices/ind_niftybanklist.csv",
	"NSE:NIFTY NEXT 50": "https://archives.nseindia.com/content/indices/ind_niftynext50list.csv",
	// "NSE:NIFTY MIDCAP 50":    "https://archives.nseindia.com/content/indices/ind_niftymidcap50list.csv",
	// "NSE:NIFTY MIDCAP 100":   "https://archives.nseindia.com/content/indices/ind_niftymidcap100list.csv",
	// "NSE:NIFTY SMALLCAP 100": "https://archives.nseindia.com/content/indices/ind_niftysmallcap100list.csv",
	// "NSE:NIFTY IT":           "https://archives.nseindia.com/content/indices/ind_niftyitlist.csv",
	// "NSE:NIFTY AUTO":         "https://archives.nseindia.com/content/indices/ind_niftyautolist.csv",
	// "NSE:NIFTY FMCG":         "https://archives.nseindia.com/content/indices/ind_niftyfmcglist.csv",
	// "NSE:NIFTY PHARMA":       "https://archives.nseindia.com/content/indices/ind_niftypharmalist.csv",
	// "NSE:NIFTY METAL":        "https://archives.nseindia.com/content/indices/ind_niftymetallist.csv",
}

// IndexService is the service for managing indices
type IndexService struct {
	client *http.Client
	repo   *Repository
	state  *state.State
	logger *logger.Logger
}

// NewIndexService creates a new IndexService
func NewIndexService(db *gorm.DB) *IndexService {
	stateManager, err := state.NewState(db)
	if err != nil {
		zaplogger.Fatal("failed to create state manager", zaplogger.Fields{"error": err})
	}
	logger, err := logger.New(db, "INDEX SERVICE")
	if err != nil {
		zaplogger.Error("failed to create cron logger", zaplogger.Fields{"error": err})
	}
	return &IndexService{
		client: &http.Client{},
		repo:   NewIndexRepository(db),
		state:  stateManager,
		logger: logger,
	}
}

// UpdateNSEIndices fetches the instruments for a given NSE index and updates the database
func (s *IndexService) UpdateNSEIndices() (int64, error) {

	// check if update is required
	lastUpdatedAt, err := s.state.Get("indices_updated_at")
	if err == nil {
		if !s.isUpdateIndicesRequired(lastUpdatedAt) {
			return 0, nil
		}
	}

	// truncate table
	if err := s.repo.TruncateIndices(); err != nil {
		s.logger.Error("Failed to truncate table", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to truncate table: %v", err)
	}

	// get instruments for all indices
	var insertedRecords int64
	for _, indexName := range s.GetNSEIndexNames() {
		// get instruments for index
		instruments, err := s.FetchNSEIndexInstruments(indexName)
		if err != nil {
			s.logger.Error("Failed to get instruments for index", map[string]interface{}{
				"index_name": indexName,
				"error":      err,
			})
			return 0, fmt.Errorf("failed to get instruments for index %s: %v", indexName, err)
		}

		// prepare indexInstruments for InsertIndices
		indexInstruments := make([]IndexModel, len(instruments))
		for i, instrument := range instruments {
			indexInstruments[i] = IndexModel{
				IndexName:  indexName,
				Instrument: instrument,
				CreatedAt:  time.Now(),
			}
		}
		count, err := s.repo.InsertIndices(indexInstruments)
		if err != nil {
			s.logger.Error("Failed to insert instruments for index", map[string]interface{}{
				"index_name": indexName,
				"error":      err,
			})
			return 0, fmt.Errorf("failed to create instruments for index %s: %v", indexName, err)
		}
		insertedRecords += count

	}

	// update state after all indices have been updated
	if err := s.state.Set("indices_updated_at", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		s.logger.Error("Failed to update state", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to update state: %v", err)
	}

	return insertedRecords, nil

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
func (s *IndexService) FetchNSEIndexInstruments(indexName string) ([]string, error) {
	url, ok := NSEIndicesURLMap[indexName]
	if !ok {
		return nil, fmt.Errorf("invalid index: %s", indexName)
	}

	// -------------------------------------------------------------------------------------------------
	// make request to index url
	// -------------------------------------------------------------------------------------------------
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		s.logger.Error("Failed to create request for index", map[string]interface{}{
			"index_name": indexName,
			"error":      err,
		})
		return nil, fmt.Errorf("failed to create request for index %s: %v", indexName, err)
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Add("referer", "https://www.nseindia.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to download CSV for index", map[string]interface{}{
			"index_name": indexName,
			"error":      err,
		})
		return nil, fmt.Errorf("failed to download CSV for index %s: %v", indexName, err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		s.logger.Error("Failed to parse CSV for index", map[string]interface{}{
			"index_name": indexName,
			"error":      err,
		})
		return nil, fmt.Errorf("failed to parse CSV for index %s: %v", indexName, err)
	}

	instruments := make([]string, 0, len(records)-1)
	for _, record := range records[1:] { // Skip header row
		if len(record) < 3 {
			continue
		}
		instruments = append(instruments, "NSE:"+record[2]) // Assuming the tradingymbol is in the third column
	}

	return instruments, nil
}

// GetNSEIndexNames returns the names of all NSE indices
func (s *IndexService) GetNSEIndexNames() []string {
	indices := make([]string, 0, len(NSEIndicesURLMap))
	for index := range NSEIndicesURLMap {
		indices = append(indices, index)
	}
	return indices
}

// GetNSEIndexInstruments fetches the instruments for a given NSE index
func (s *IndexService) GetNSEIndexInstruments(indexName string) ([]IndexModel, error) {
	return s.repo.GetNSEIndexInstruments(indexName)
}
