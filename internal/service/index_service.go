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

// var nseIndicesBaseURL = "https://nsearchives.nseindia.com/content/indices/"
// var nseIndicesBaseURL = "https://niftyindices.com/IndexConstituent/"
var nseIndicesBaseURL = "https://raw.githubusercontent.com/nsvirk/nseindicesdata/refs/heads/main/csvfiles/"

var nseIndicesFileMap = map[string]string{
	"NIFTY 50":                 "ind_nifty50list.csv",
	"NIFTY NEXT 50":            "ind_niftynext50list.csv",
	"NIFTY 100":                "ind_nifty100list.csv",
	"NIFTY 200":                "ind_nifty200list.csv",
	"NIFTY TOTAL MARKET":       "ind_niftytotalmarket_list.csv",
	"NIFTY 500":                "ind_nifty500list.csv",
	"NIFTY MIDCAP 50":          "ind_niftymidcap50list.csv",
	"NIFTY MIDCAP 100":         "ind_niftymidcap100list.csv",
	"NIFTY SMALLCAP 100":       "ind_niftysmallcap100list.csv",
	"NIFTY AUTO":               "ind_niftyautolist.csv",
	"NIFTY BANK":               "ind_niftybanklist.csv",
	"NIFTY FINANCIAL SERVICES": "ind_niftyfinancelist.csv",
	"NIFTY HEALTHCARE":         "ind_niftyhealthcarelist.csv",
	"NIFTY IT":                 "ind_niftyitlist.csv",
	"NIFTY FMCG":               "ind_niftyfmcglist.csv",
	"NIFTY METAL":              "ind_niftymetallist.csv",
	"NIFTY PHARMA":             "ind_niftypharmalist.csv",
	"NIFTY REALTY":             "ind_niftyrealtylist.csv",
	"NIFTY CONSUMER DURABLES":  "ind_niftyconsumerdurableslist.csv",
	"NIFTY OIL GAS":            "ind_niftyoilgaslist.csv",
}

var nseIndicesUpdatedAtKey = "NSE_INDICES_UPDATED_AT"

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

// GetAllIndices returns all indices
func (s *IndexService) GetAllIndices() ([]models.IndexModel, error) {
	return s.repo.GetAllIndices()
}

// GetIndicesByExchange returns the names of all indices for a given exchange
func (s *IndexService) GetIndicesByExchange(exchange string) ([]models.IndexModel, error) {
	return s.repo.GetIndicesByExchange(exchange)
}

// GetIndexInstruments returns the instruments for a given index
func (s *IndexService) GetIndexInstruments(exchange, index string) ([]models.InstrumentModel, error) {
	indexRecords, err := s.repo.GetIndexInstruments(exchange, index)
	if err != nil {
		return nil, err
	}
	// get instruments from index repo
	indexTradingsymbols := make([]string, len(indexRecords))
	for i, indexRecord := range indexRecords {
		indexTradingsymbols[i] = indexRecord.Tradingsymbol
	}
	// get instruments from instrument repo
	instruments, err := s.instrumentRepo.GetInstrumentByExchangeTradingsymbols(exchange, indexTradingsymbols)
	if err != nil {
		return nil, err
	}
	return instruments, nil
}

// UpdateIndices updates the indices in the database
func (s *IndexService) UpdateIndices() (int64, error) {
	var grandTotalInserted int64
	// update NSE indices
	totalInserted, err := s.updateNSEIndices()
	if err != nil {
		return 0, fmt.Errorf("failed to update NSE indices: %v", err)
	}
	grandTotalInserted += totalInserted

	// Update Other indices
	// ToDo

	return grandTotalInserted, nil
}

// UpdateNSEIndices fetches the instruments for a given NSE index and updates the database
func (s *IndexService) updateNSEIndices() (int64, error) {

	// check if update is required
	nseIndicesUpdatedAtValue, err := s.state.Get(nseIndicesUpdatedAtKey)
	if err == nil {
		if !s.isUpdateIndicesRequired(nseIndicesUpdatedAtValue) {
			zaplogger.Info("Indices update not required", zaplogger.Fields{
				nseIndicesUpdatedAtKey: nseIndicesUpdatedAtValue,
			})
			return 0, nil
		}
	}

	// update log with logger
	zaplogger.Info("Indices update required", zaplogger.Fields{
		nseIndicesUpdatedAtKey: nseIndicesUpdatedAtValue,
	})

	// truncate table
	if err := s.repo.TruncateIndicesTable(); err != nil {
		return 0, fmt.Errorf("failed to truncate table: %v", err)
	}

	// get instruments for all indices
	var totalInserted int64
	var indices []string
	for index := range nseIndicesFileMap {
		indices = append(indices, index)
	}

	if len(indices) == 0 {
		return 0, fmt.Errorf("no indices found in nseIndicesFileMap")
	}

	// update indices
	for _, index := range indices {
		// get records for index
		indexRecords, err := s.fetchNSEIndexInstruments(index)
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
	if err := s.state.Set(nseIndicesUpdatedAtKey, time.Now().Format("2006-01-02 15:04:05")); err != nil {
		return 0, fmt.Errorf("failed to update state: %v", err)
	}

	zaplogger.Info("NSE Indices updated", zaplogger.Fields{
		"totalInserted": totalInserted,
	})

	return totalInserted, nil
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

// fetchNSEIndexInstruments fetches the instruments for a given NSE index
func (s *IndexService) fetchNSEIndexInstruments(index string) ([]models.IndexModel, error) {

	// -------------------------------------------------------------------------------------------------
	// make request to index url
	// -------------------------------------------------------------------------------------------------
	// get index csv file name
	indexCsvFile, ok := nseIndicesFileMap[index]
	if !ok {
		return nil, fmt.Errorf("invalid index: %s", index)
	}

	// make url
	url := fmt.Sprintf("%s%s", nseIndicesBaseURL, indexCsvFile)

	// create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for index %s: %v", index, err)
	}

	// set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Referer", "https://niftyindices.com/")

	// make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download CSV for index %s: %v", index, err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV for index %s: %v", index, err)
	}

	indexRecords := make([]models.IndexModel, 0, len(records)-1)
	for _, record := range records[1:] { // Skip header row
		// record : [Company Name, Industry, Symbol, Series, ISIN Code]
		if len(record) < 5 {
			continue
		}
		indexRecords = append(indexRecords, models.IndexModel{
			Index:         index,
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
