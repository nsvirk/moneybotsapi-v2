// File: github.com/nsvirk/moneybotsapi/instrument/index_service.go

package instrument

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/nsvirk/moneybotsapi/shared/applogger"
)

var IndexURLMap = map[string]string{
	"NSE:NIFTY 50":           "https://archives.nseindia.com/content/indices/ind_nifty50list.csv",
	"NSE:NIFTY NEXT 50":      "https://archives.nseindia.com/content/indices/ind_niftynext50list.csv",
	"NSE:NIFTY 100":          "https://archives.nseindia.com/content/indices/ind_nifty100list.csv",
	"NSE:NIFTY 200":          "https://archives.nseindia.com/content/indices/ind_nifty200list.csv",
	"NSE:NIFTY 500":          "https://archives.nseindia.com/content/indices/ind_nifty500list.csv",
	"NSE:NIFTY MIDCAP 50":    "https://archives.nseindia.com/content/indices/ind_niftymidcap50list.csv",
	"NSE:NIFTY MIDCAP 100":   "https://archives.nseindia.com/content/indices/ind_niftymidcap100list.csv",
	"NSE:NIFTY SMALLCAP 100": "https://archives.nseindia.com/content/indices/ind_niftysmallcap100list.csv",
	"NSE:NIFTY BANK":         "https://archives.nseindia.com/content/indices/ind_niftybanklist.csv",
	"NSE:NIFTY IT":           "https://archives.nseindia.com/content/indices/ind_niftyitlist.csv",
	"NSE:NIFTY AUTO":         "https://archives.nseindia.com/content/indices/ind_niftyautolist.csv",
	"NSE:NIFTY FMCG":         "https://archives.nseindia.com/content/indices/ind_niftyfmcglist.csv",
	"NSE:NIFTY PHARMA":       "https://archives.nseindia.com/content/indices/ind_niftypharmalist.csv",
	"NSE:NIFTY METAL":        "https://archives.nseindia.com/content/indices/ind_niftymetallist.csv",
}

type IndexService struct {
	client *http.Client
}

func NewIndexService() *IndexService {
	return &IndexService{
		client: &http.Client{},
	}
}

func (s *IndexService) FetchIndexInstrumentsList(indexName string) ([]string, error) {
	url, ok := IndexURLMap[indexName]
	applogger.Info("Fetching index instruments list for index: " + indexName)
	if !ok {
		return nil, fmt.Errorf("invalid index: %s", indexName)
	}

	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download CSV for index %s: %v", indexName, err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV for index %s: %v", indexName, err)
	}

	symbols := make([]string, 0, len(records)-1)
	for _, record := range records[1:] { // Skip header row
		symbols = append(symbols, "NSE:"+record[2]) // Assuming the symbol is in the third column
	}

	return symbols, nil
}

func (s *IndexService) GetAvailableIndices() []string {
	indices := make([]string, 0, len(IndexURLMap))
	for index := range IndexURLMap {
		indices = append(indices, index)
	}
	return indices
}
