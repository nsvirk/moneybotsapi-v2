// Package service contains the service layer for the Moneybots API
package service

import (
	"fmt"
	"log"

	"github.com/nsvirk/moneybotsapi/internal/models"
	"gorm.io/gorm"
)

// QuoteService is the service for the quote API
type QuoteService struct {
	db *gorm.DB
}

// NewQuoteService creates a new quote service
func NewQuoteService(db *gorm.DB) *QuoteService {
	return &QuoteService{db: db}
}

// GetTickData gets the tick data for the given instruments
func (s *QuoteService) GetTickData(instruments []string) (map[string]*models.TickerData, error) {
	var tickerData []models.TickerData
	err := s.db.Where("instrument IN ?", instruments).Find(&tickerData).Error
	if err != nil {
		log.Printf("Database query error: %v", err)
		return nil, fmt.Errorf("error fetching tick data from database: %v", err)
	}

	return s.createTickerDataMap(tickerData, instruments)
}

// createTickerDataMap creates a map of ticker data for the given instruments
func (s *QuoteService) createTickerDataMap(tickerData []models.TickerData, instruments []string) (map[string]*models.TickerData, error) {
	if len(tickerData) == 0 {
		log.Printf("No tick data found for instruments: %v", instruments)
		return nil, fmt.Errorf("no tick data found for any of the requested instruments")
	}

	tickerDataMap := make(map[string]*models.TickerData)
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
