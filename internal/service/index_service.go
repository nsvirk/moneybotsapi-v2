// Package service contains the service layer for the Moneybots API
package service

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/repository"
	"github.com/nsvirk/moneybotsapi/pkg/utils/logger"
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
	client *http.Client
	repo   *repository.IndexRepository
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
		repo:   repository.NewIndexRepository(db),
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
			// update log with logger
			s.logger.Info("Indices update not required", map[string]interface{}{
				"lastUpdatedAt": lastUpdatedAt,
			})
			return 0, nil
		}
	}
	// update log with logger
	s.logger.Info("Indices update required", map[string]interface{}{
		"lastUpdatedAt": lastUpdatedAt,
	})

	// fetch nse india home page
	if err := s.fetchNseIndiaHomePage(); err != nil {
		s.logger.Error("Failed to fetch nseindia.com home page", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to fetch nseindia.com home page: %v", err)
	}

	// truncate table
	if err := s.repo.TruncateIndices(); err != nil {
		s.logger.Error("Failed to truncate table", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to truncate table: %v", err)
	}

	// get instruments for all indices
	var totalInserted int64
	indices, err := s.GetNSEIndexNamesFromNSEIndicesFileMap()
	if err != nil {
		s.logger.Error("Failed to get indices", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to get indices: %v", err)
	}

	// update indices
	for _, index := range indices {
		// get instruments for index
		indexInstruments, err := s.FetchNSEIndexInstruments(index)
		if err != nil {
			s.logger.Error("Failed to get instruments for index", map[string]interface{}{
				"indexindex": index,
				"error":      err,
			})
			return 0, fmt.Errorf("failed to get instruments for index %s: %v", index, err)
		}

		// prepare indexInstruments for InsertIndices
		for i, idxInstrument := range indexInstruments {
			indexInstruments[i] = models.IndexModel{
				Index:       index,
				Instrument:  idxInstrument.Instrument,
				CompanyName: idxInstrument.CompanyName,
				Industry:    idxInstrument.Industry,
				Series:      idxInstrument.Series,
				ISINCode:    idxInstrument.ISINCode,
				CreatedAt:   time.Now(),
			}
		}
		count, err := s.repo.InsertIndices(indexInstruments)
		if err != nil {
			s.logger.Error("Failed to insert instruments for index", map[string]interface{}{
				"index": index,
				"error": err,
			})
			return 0, fmt.Errorf("failed to create instruments for index %s: %v", index, err)
		}
		totalInserted += count

	}

	// update state after all indices have been updated
	if err := s.state.Set("indices_updated_at", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		s.logger.Error("Failed to update state", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to update state: %v", err)
	}

	// update log with logger
	s.logger.Info("Indices updated", map[string]interface{}{
		"totalInserted": totalInserted,
	})

	// get indices record count
	recordCount, err := s.repo.GetIndicesRecordCount()
	if err != nil {
		s.logger.Error("Failed to get indices record count", map[string]interface{}{
			"error": err,
		})
		return 0, fmt.Errorf("failed to get indices record count: %v", err)
	}

	// insert record count in logs
	s.logger.Info("Indices record count", map[string]interface{}{
		"recordCount": recordCount,
	})

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

func (s *IndexService) fetchNseIndiaHomePage() error {
	// -------------------------------------------------------------------------------------------------
	// make request to https://nseindia.com/
	// -------------------------------------------------------------------------------------------------
	baseUrl := "https://nseindia.com"
	req, err := http.NewRequest("GET", baseUrl, nil)
	if err != nil {
		s.logger.Error("Failed to create request for nseindia.com", map[string]interface{}{
			"url":   baseUrl,
			"error": err,
		})
		return fmt.Errorf("failed to create request for nseindia.com %s: %v", baseUrl, err)
	}

	// set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	_, err = s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to make request to nseindia.com", map[string]interface{}{
			"url":   baseUrl,
			"error": err,
		})
		return fmt.Errorf("failed to make request to nseindia.com %s: %v", baseUrl, err)
	}

	return nil
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
		s.logger.Error("Failed to create request for index", map[string]interface{}{
			"index_name": indexName,
			"error":      err,
		})
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

	indexInstruments := make([]models.IndexModel, 0, len(records)-1)
	for _, record := range records[1:] { // Skip header row
		// record is [Company Name, Industry, Symbol, Series, ISIN Code]
		//  sample is 360 ONE WAM Ltd, Financial Services, 360ONE, EQ, INE466L01038
		if len(record) < 5 {
			continue
		}
		indexInstruments = append(indexInstruments, models.IndexModel{
			Index:       indexName,
			Instrument:  "NSE:" + record[2],
			CompanyName: record[0],
			Industry:    record[1],
			Series:      record[3],
			ISINCode:    record[4],
		})
	}

	return indexInstruments, nil
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
func (s *IndexService) GetNSEIndexInstruments(indexName string) ([]string, error) {
	instruments, err := s.repo.GetNSEIndexInstruments(indexName)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(instruments))
	for i, instrument := range instruments {
		result[i] = instrument.Instrument
	}

	return result, nil
}
