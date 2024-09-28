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

// TruncateInstrumentsTable truncates the instruments table
func (r *InstrumentRepository) TruncateInstrumentsTable() error {
	return r.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", models.InstrumentsTableName)).Error
}

// InsertInstruments inserts a batch of instruments into the database
func (r *InstrumentRepository) InsertInstruments(records [][]string) (int64, error) {
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

	stmt := fmt.Sprintf("INSERT INTO %s (instrument_token, exchange_token, tradingsymbol, name, last_price, expiry, strike, tick_size, lot_size, instrument_type, segment, exchange, updated_at) VALUES %s",
		models.InstrumentsTableName,
		strings.Join(valueStrings, ","),
	)

	result := r.DB.Exec(stmt, valueArgs...)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to insert batch into %s: %v", models.InstrumentsTableName, result.Error)
	}

	return result.RowsAffected, nil
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
func (r *InstrumentRepository) QueryInstruments(qip models.QueryInstrumentsParams) ([]models.InstrumentModel, error) {

	query := r.DB.Model(&models.InstrumentModel{})

	if qip.Exchange != "" {
		query = query.Where("exchange = ?", qip.Exchange)
	}

	if qip.Tradingsymbol != "" {
		query = query.Where("tradingsymbol = ?", qip.Tradingsymbol)
	}

	if qip.Name != "" {
		query = query.Where("name = ?", qip.Name)
	}

	if qip.Expiry != "" {
		query = query.Where("expiry = ?", qip.Expiry)
	}

	if qip.Strike != "" {
		strikeFloat, err := strconv.ParseFloat(qip.Strike, 64)
		if err != nil {
			return nil, err
		}
		query = query.Where("strike = ?", strikeFloat)
	}

	if qip.Segment != "" {
		query = query.Where("segment = ?", qip.Segment)
	}

	if qip.InstrumentType != "" {
		query = query.Where("instrument_type = ?", qip.InstrumentType)
	}

	var instruments []models.InstrumentModel
	if err := query.Find(&instruments).Error; err != nil {
		return nil, err
	}

	return instruments, nil
}

// GetExchangeNamesByExpiry queries the instruments table by expiry and returns a list of distinct exchange, names
func (r *InstrumentRepository) GetExchangeNamesByExpiry(expiry string) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	err := r.DB.Model(&models.InstrumentModel{}).
		Select("DISTINCT exchange, name").
		Where("expiry = ?", expiry).
		Find(&instruments).
		Error
	return instruments, err
}

// GetInstrumentsByTradingsymbol gets instruments by tradingsymbol
func (r *InstrumentRepository) GetInstrumentsByTradingsymbol(tradingsymbol string) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	if err := r.DB.Where("tradingsymbol = ?", tradingsymbol).Find(&instruments).Error; err != nil {
		return nil, err
	}
	return instruments, nil
}

// GetInstrumentsByInstrumentToken gets instruments by instrument token
func (r *InstrumentRepository) GetInstrumentsByInstrumentToken(instrumentToken string) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	if err := r.DB.Where("instrument_token = ?", instrumentToken).Find(&instruments).Error; err != nil {
		return nil, err
	}
	return instruments, nil
}

// GetInstrumentsByExpiry gets instruments by expiry
func (r *InstrumentRepository) GetInstrumentsByExpiry(expiry string) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	if err := r.DB.Where("expiry = ?", expiry).Find(&instruments).Error; err != nil {
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

// GetInstrumentsByExchange gets instruments by exchange
func (r *InstrumentRepository) GetInstrumentsByExchange(exchange string) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	if err := r.DB.Where("exchange = ?", exchange).Find(&instruments).Error; err != nil {
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

// GetInstrumentByExchangeTradingsymbols gets an instrument by exchange and tradingsymbols
func (r *InstrumentRepository) GetInstrumentByExchangeTradingsymbols(exchange string, tradingsymbols []string) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	err := r.DB.Where("exchange = ? AND tradingsymbol IN (?)", exchange, tradingsymbols).Find(&instruments).Error
	return instruments, err
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
