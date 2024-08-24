package quote

import (
	"fmt"
	"log"

	"github.com/nsvirk/moneybotsapi/ticker"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetTickData(instruments []string) (map[string]*ticker.TickerData, error) {
	var tickerData []ticker.TickerData
	err := s.db.Where("instrument IN ?", instruments).Find(&tickerData).Error
	if err != nil {
		log.Printf("Database query error: %v", err)
		return nil, fmt.Errorf("error fetching tick data from database: %v", err)
	}

	return createTickerDataMap(tickerData, instruments)
}

func createTickerDataMap(tickerData []ticker.TickerData, instruments []string) (map[string]*ticker.TickerData, error) {
	if len(tickerData) == 0 {
		log.Printf("No tick data found for instruments: %v", instruments)
		return nil, fmt.Errorf("no tick data found for any of the requested instruments")
	}

	tickerDataMap := make(map[string]*ticker.TickerData)
	for i := range tickerData {
		tickerDataMap[tickerData[i].Instrument] = &tickerData[i]
	}

	missingInstruments := []string{}
	for _, instrument := range instruments {
		if _, ok := tickerDataMap[instrument]; !ok {
			missingInstruments = append(missingInstruments, instrument)
		}
	}

	if len(missingInstruments) > 0 {
		log.Printf("No data found for instruments: %v", missingInstruments)
	}

	return tickerDataMap, nil
}
