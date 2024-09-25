// Package repository contains the repository layer for the Moneybots API
package repository

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nsvirk/moneybotsapi/internal/models"
	"gorm.io/gorm"
)

// InstrumentRepository is the database repository for instruments
type InstrumentRepository struct {
	DB *gorm.DB
}

// NewInstrumentRepository creates a new instrument repository
func NewInstrumentRepository(db *gorm.DB) *InstrumentRepository {
	return &InstrumentRepository{DB: db}
}

// TruncateInstruments truncates the instruments table
func (r *InstrumentRepository) TruncateInstruments() error {
	return r.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", models.InstrumentsTableName)).Error
}

// InsertInstruments inserts a batch of instruments into the database
func (r *InstrumentRepository) InsertInstruments(records [][]string) (int, error) {
	valueStrings := make([]string, 0, len(records))
	valueArgs := make([]interface{}, 0, len(records)*13)

	now := time.Now().Format("2006-01-02 15:04:05")

	for _, record := range records {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		instrumentToken, _ := strconv.ParseUint(record[0], 10, 32)
		exchangeToken, _ := strconv.ParseUint(record[1], 10, 32)
		lastPrice, _ := strconv.ParseFloat(record[4], 64)
		strike, _ := strconv.ParseFloat(record[6], 64)
		tickSize, _ := strconv.ParseFloat(record[7], 64)
		lotSize, _ := strconv.ParseUint(record[8], 10, 32)

		valueArgs = append(valueArgs,
			uint(instrumentToken),
			uint(exchangeToken),
			record[2],
			record[3],
			lastPrice,
			record[5],
			strike,
			tickSize,
			uint(lotSize),
			record[9],
			record[10],
			record[11],
			now,
		)
	}

	stmt := fmt.Sprintf("INSERT INTO %s (instrument_token, exchange_token, tradingsymbol, name, last_price, expiry, strike, tick_size, lot_size, instrument_type, segment, exchange, created_at) VALUES %s",
		models.InstrumentsTableName,
		strings.Join(valueStrings, ","),
	)

	result := r.DB.Exec(stmt, valueArgs...)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to insert batch into %s: %v", models.InstrumentsTableName, result.Error)
	}

	return int(result.RowsAffected), nil
}

// GetInstrumentsRecordCount returns the number of records in the instruments table
func (r *InstrumentRepository) GetInstrumentsRecordCount() (int64, error) {
	var count int64
	err := r.DB.Table(models.InstrumentsTableName).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get instruments record count: %v", err)
	}
	return count, nil
}

// QueryInstruments queries the instruments table
func (r *InstrumentRepository) QueryInstruments(exchange, tradingsymbol, expiry, strike, segment string) ([]models.InstrumentModel, error) {
	query := r.DB.Model(&models.InstrumentModel{})

	if exchange != "" {
		query = query.Where("exchange LIKE ?", exchange)
	}
	if tradingsymbol != "" {
		query = query.Where("tradingsymbol LIKE ?", tradingsymbol)
	}
	if expiry != "" {
		query = query.Where("expiry LIKE ?", expiry)
	}
	if strike != "" {
		strikeFloat, err := strconv.ParseFloat(strike, 64)
		if err != nil {
			return nil, err
		}
		query = query.Where("strike = ?", strikeFloat)
	}

	if segment != "" {
		query = query.Where("segment LIKE ?", segment)
	}

	var instruments []models.InstrumentModel
	if err := query.Find(&instruments).Error; err != nil {
		return nil, err
	}

	return instruments, nil
}

// GetInstrumentsByTokens gets instruments by tokens
func (r *InstrumentRepository) GetInstrumentsByTokens(tokens []uint32) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	if err := r.DB.Where("instrument_token IN ?", tokens).Find(&instruments).Error; err != nil {
		return nil, err
	}
	return instruments, nil
}

// GetInstrumentByExchangeTradingsymbol gets an instrument by exchange and tradingsymbol
func (r *InstrumentRepository) GetInstrumentByExchangeTradingsymbol(exchange, tradingsymbol string) (models.InstrumentModel, error) {
	var instrument models.InstrumentModel
	err := r.DB.Where("exchange = ? AND tradingsymbol = ?", exchange, tradingsymbol).First(&instrument).Error
	return instrument, err
}

// GetOptionChainNames returns a list of exchange:name for a given expiry
func (r *InstrumentRepository) GetOptionChainNames(expiry string) ([]string, error) {
	var exchangeNames []string
	err := r.DB.Model(&models.InstrumentModel{}).
		Select("DISTINCT CONCAT(exchange, ':', name) AS exchange_name").
		Where("expiry = ?", expiry).
		Pluck("exchange_name", &exchangeNames).
		Error
	return exchangeNames, err
}

// GetOptionChainInstruments returns a list of instruments for a given exchange, name and expiry
func (r *InstrumentRepository) GetOptionChainInstruments(exchange, name, expiry string) ([]models.InstrumentModel, error) {
	// get fut instruments
	var futInstruments []models.InstrumentModel
	instrumentType := "FUT"
	err := r.DB.Where("instrument_type = ? AND exchange = ? AND name = ? AND expiry >= ? ORDER BY expiry ASC LIMIT 1", instrumentType, exchange, name, expiry).Find(&futInstruments).Error
	if err != nil {
		return nil, err
	}

	// get the opt instruments
	var optInstruments []models.InstrumentModel
	instrumentTypes := []string{"CE", "PE"}
	err = r.DB.Where("instrument_type IN (?) AND exchange = ? AND name = ? AND expiry = ?", instrumentTypes, exchange, name, expiry).Find(&optInstruments).Error
	if err != nil {
		return nil, err
	}

	// append the instruments
	ocInstruments := append(futInstruments, optInstruments...)

	return ocInstruments, nil

}
