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

// GetInstrumentsQuery queries the instruments table
func (r *InstrumentRepository) GetInstrumentsQuery(qip models.QueryInstrumentsParams) ([]models.InstrumentModel, error) {

	query := r.DB.Model(&models.InstrumentModel{})

	if qip.Exchange != "" {
		query = query.Where("exchange = ?", qip.Exchange)
	}

	if qip.Tradingsymbol != "" {
		query = query.Where("tradingsymbol = ?", qip.Tradingsymbol)
	}

	if qip.InstrumentToken != "" {
		instrumentToken, err := strconv.ParseUint(qip.InstrumentToken, 10, 32)
		if err != nil {
			return nil, err
		}
		query = query.Where("instrument_token = ?", uint32(instrumentToken))
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

// GetInstrumentsByExchange gets instruments by exchange
func (r *InstrumentRepository) GetInstrumentsByExchange(exchange string) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	if err := r.DB.Where("exchange = ?", exchange).Find(&instruments).Error; err != nil {
		return nil, err
	}
	return instruments, nil
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

// GetInstrumentsByTokens returns instruments by tokens
func (r *InstrumentRepository) GetInstrumentsByTokens(tokens []uint32) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	if err := r.DB.Where("instrument_token IN ?", tokens).Find(&instruments).Error; err != nil {
		return nil, err
	}
	return instruments, nil
}

// GetFNOSegmentWiseName returns a list of segment wise name for a given expiry
func (r *InstrumentRepository) GetFNOSegmentWiseName(expiry string) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	err := r.DB.Model(&models.InstrumentModel{}).
		Select("DISTINCT segment, name").
		Where("expiry = ?", expiry).
		Find(&instruments).
		Error
	return instruments, err
}

// GetFNOSegmentWiseExpiry returns a list of segment wise expiry for a given name
func (r *InstrumentRepository) GetFNOSegmentWiseExpiry(name string, limit, offset int) ([]models.InstrumentModel, error) {
	var instruments []models.InstrumentModel
	err := r.DB.Model(&models.InstrumentModel{}).
		Select("DISTINCT segment, expiry").
		Where("name = ? ", name).
		Order("expiry ASC").
		Limit(limit).
		Offset(offset).
		Find(&instruments).
		Error
	return instruments, err
}

// GetFNOOptionChain returns the option chain for a given instrument
func (r *InstrumentRepository) GetFNOOptionChain(exchange, name, futExpiry, optExpiry string) ([]models.InstrumentModel, error) {
	// get fut instruments
	var futInstruments []models.InstrumentModel
	if len(futExpiry) > 0 {
		err := r.DB.Where("exchange = ? AND name = ? AND expiry = ? AND instrument_type = 'FUT' ORDER BY expiry ASC LIMIT 1", exchange, name, futExpiry).
			Find(&futInstruments).
			Error
		if err != nil {
			return nil, err
		}
	}

	// get opt instruments
	var optInstruments []models.InstrumentModel
	err := r.DB.Where("exchange = ? AND name = ? AND expiry = ? AND instrument_type IN ('CE', 'PE')", exchange, name, optExpiry).
		Find(&optInstruments).
		Error
	if err != nil {
		return nil, err
	}

	return append(futInstruments, optInstruments...), nil
}
