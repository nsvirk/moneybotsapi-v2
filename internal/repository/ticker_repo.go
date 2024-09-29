// Package repository contains the repository layer for the Moneybots API
package repository

import (
	"fmt"
	"time"

	"github.com/nsvirk/moneybotsapi/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TickerRepository struct {
	DB *gorm.DB
}

// NewTickerRepository creates a new TickerRepository
func NewTickerRepository(db *gorm.DB) *TickerRepository {
	return &TickerRepository{DB: db}
}

// --------------------------------------------
// TickerInstruments func's grouped together
// --------------------------------------------
// TruncateTickerInstruments truncates the ticker instruments
func (r *TickerRepository) TruncateTickerInstruments() (int64, error) {
	// Start a transaction
	tx := r.DB.Begin()
	if tx.Error != nil {
		return 0, fmt.Errorf("failed to begin transaction: %v", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Count the rows
	var count int64
	if err := tx.Table(models.TickerInstrumentsTableName).Count(&count).Error; err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("failed to count rows in %s: %v", models.TickerInstrumentsTableName, err)
	}

	// Truncate the table
	if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s;", models.TickerInstrumentsTableName)).Error; err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("failed to truncate table %s: %v", models.TickerInstrumentsTableName, err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return count, nil
}

// UpsertTickerInstruments upserts the instruments
func (r *TickerRepository) UpsertTickerInstruments(userID string, instruments []models.InstrumentModel) (int64, int64, error) {
	var insertedCount int64
	var updatedCount int64

	for _, instrument := range instruments {
		result := r.DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "instrument"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"instrument_token", "updated_at"}),
		}).Create(&models.TickerInstrument{
			UserID:          userID,
			Instrument:      instrument.Exchange + ":" + instrument.Tradingsymbol,
			InstrumentToken: uint32(instrument.InstrumentToken),
			UpdatedAt:       time.Now(),
		})

		if result.Error != nil {
			return 0, 0, fmt.Errorf("error upserting instrument: %v", result.Error)
		}

		if result.RowsAffected == 1 {
			insertedCount++
		} else {
			updatedCount++
		}
	}

	return insertedCount, updatedCount, nil
}

// GetTickerInstruments gets the ticker instruments
func (r *TickerRepository) GetTickerInstruments(userID string) ([]models.TickerInstrument, error) {
	var tickerInstruments []models.TickerInstrument
	err := r.DB.Where("user_id = ?", userID).Find(&tickerInstruments).Error
	return tickerInstruments, err
}

// GetTickerInstrumentCount gets the ticker instrument count
func (r *TickerRepository) GetTickerInstrumentCount(userID string) (int64, error) {
	var count int64
	err := r.DB.Model(&models.TickerInstrument{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// DeleteTickerInstruments deletes the ticker instruments
func (r *TickerRepository) DeleteTickerInstruments(userID string, instruments []string) (int64, error) {
	result := r.DB.Where("user_id = ? AND instrument IN ?", userID, instruments).Delete(&models.TickerInstrument{})
	return result.RowsAffected, result.Error
}

// --------------------------------------------
// TickerData func's grouped together
// --------------------------------------------
// TruncateTickerData truncates the ticker data
func (r *TickerRepository) TruncateTickerData() error {
	result := r.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", models.TickerDataTableName))
	if result.Error != nil {
		return fmt.Errorf("failed to truncate table %s: %v", models.TickerDataTableName, result.Error)
	}
	return nil
}

// UpsertTickerData upserts the ticker data
func (r *TickerRepository) UpsertTickerData(tickerData []models.TickerData) error {
	if len(tickerData) == 0 {
		return nil
	}

	deduplicatedData := make(map[uint32]models.TickerData)
	for _, data := range tickerData {
		if existing, ok := deduplicatedData[data.InstrumentToken]; !ok || existing.UpdatedAt.Before(data.UpdatedAt) {
			deduplicatedData[data.InstrumentToken] = data
		}
	}

	uniqueTickerData := make([]models.TickerData, 0, len(deduplicatedData))
	for _, data := range deduplicatedData {
		uniqueTickerData = append(uniqueTickerData, data)
	}

	// upsert in each field
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		for _, data := range uniqueTickerData {
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "instrument_token"}},
				DoUpdates: clause.AssignmentColumns([]string{"timestamp", "last_trade_time", "last_price", "last_traded_quantity", "total_buy_quantity", "total_sell_quantity", "volume", "average_price", "oi", "oi_day_high", "oi_day_low", "net_change", "ohlc", "depth", "updated_at"}),
			}).Create(&data)

			if result.Error != nil {
				return fmt.Errorf("failed to upsert ticker data for instrument %s: %v", data.Instrument, result.Error)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to upsert ticker data: %v", err)
	}

	return nil
}

// --------------------------------------------
// TickerLog func's grouped together
// --------------------------------------------

// log logs a message
func (r *TickerRepository) log(level models.LogLevel, eventType, message string) error {
	timestamp := time.Now()
	log := models.TickerLog{
		Timestamp: &timestamp,
		Level:     &level,
		EventType: &eventType,
		Message:   &message,
	}
	return r.DB.Create(&log).Error
}

// Debug logs a debug message
func (r *TickerRepository) Debug(eventType, message string) error {
	return r.log(models.DEBUG, eventType, message)
}

// Info logs an info message
func (r *TickerRepository) Info(eventType, message string) error {
	return r.log(models.INFO, eventType, message)
}

// Warn logs a warning message
func (r *TickerRepository) Warn(eventType, message string) error {
	return r.log(models.WARN, eventType, message)
}

// Error logs an error message
func (r *TickerRepository) Error(eventType, message string) error {
	return r.log(models.ERROR, eventType, message)
}

// Fatal logs a fatal message
func (r *TickerRepository) Fatal(eventType, message string) error {
	return r.log(models.FATAL, eventType, message)
}

// --------------------------------------------
// Other funcs
// --------------------------------------------
func (r *TickerRepository) GetInstrumentToken(exchange, symbol string) (uint32, error) {
	var instrument models.InstrumentModel
	err := r.DB.Where("exchange = ? AND tradingsymbol = ?", exchange, symbol).First(&instrument).Error
	if err != nil {
		return 0, err
	}
	return uint32(instrument.InstrumentToken), nil
}
