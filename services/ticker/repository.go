package ticker

import (
	"fmt"
	"time"

	"github.com/nsvirk/moneybotsapi/services/instrument"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	DB *gorm.DB
}

// NewRepository creates a new Repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// --------------------------------------------
// TickerInstruments func's grouped together
// --------------------------------------------
// TruncateTickerInstruments truncates the ticker instruments
func (r *Repository) TruncateTickerInstruments() (int64, error) {
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
	if err := tx.Table(TickerInstrumentsTableName).Count(&count).Error; err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("failed to count rows in %s: %v", TickerInstrumentsTableName, err)
	}

	// Truncate the table
	if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s;", TickerInstrumentsTableName)).Error; err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("failed to truncate table %s: %v", TickerInstrumentsTableName, err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return count, nil
}

// UpsertTickerInstruments upserts the instruments
func (r *Repository) UpsertTickerInstruments(userID string, instrumentsTokenMap map[string]uint32) (int, int, error) {
	var insertedCount int
	var updatedCount int

	for instrument, token := range instrumentsTokenMap {
		result := r.DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "instrument"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"instrument_token", "updated_at"}),
		}).Create(&TickerInstrument{
			UserID:          userID,
			Instrument:      instrument,
			InstrumentToken: token,
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
func (r *Repository) GetTickerInstruments(userID string) ([]TickerInstrument, error) {
	var tickerInstruments []TickerInstrument
	err := r.DB.Where("user_id = ?", userID).Find(&tickerInstruments).Error
	return tickerInstruments, err
}

// GetTickerInstrumentCount gets the ticker instrument count
func (r *Repository) GetTickerInstrumentCount(userID string) (int64, error) {
	var count int64
	err := r.DB.Model(&TickerInstrument{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// DeleteTickerInstruments deletes the ticker instruments
func (r *Repository) DeleteTickerInstruments(userID string, instruments []string) (int64, error) {
	result := r.DB.Where("user_id = ? AND instrument IN ?", userID, instruments).Delete(&TickerInstrument{})
	return result.RowsAffected, result.Error
}

// --------------------------------------------
// TickerData func's grouped together
// --------------------------------------------
// TruncateTickerData truncates the ticker data
func (r *Repository) TruncateTickerData() error {
	result := r.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", TickerDataTableName))
	if result.Error != nil {
		return fmt.Errorf("failed to truncate table %s: %v", TickerDataTableName, result.Error)
	}
	return nil
}

func (r *Repository) UpsertTickerData(tickerData []TickerData) error {
	if len(tickerData) == 0 {
		return nil
	}

	deduplicatedData := make(map[uint32]TickerData)
	for _, data := range tickerData {
		if existing, ok := deduplicatedData[data.InstrumentToken]; !ok || existing.UpdatedAt.Before(data.UpdatedAt) {
			deduplicatedData[data.InstrumentToken] = data
		}
	}

	uniqueTickerData := make([]TickerData, 0, len(deduplicatedData))
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
func (r *Repository) log(level LogLevel, eventType, message string) error {
	timestamp := time.Now()
	log := TickerLog{
		Timestamp: &timestamp,
		Level:     &level,
		EventType: &eventType,
		Message:   &message,
	}
	return r.DB.Create(&log).Error
}

// Debug logs a debug message
func (r *Repository) Debug(eventType, message string) error {
	return r.log(DEBUG, eventType, message)
}

// Info logs an info message
func (r *Repository) Info(eventType, message string) error {
	return r.log(INFO, eventType, message)
}

// Warn logs a warning message
func (r *Repository) Warn(eventType, message string) error {
	return r.log(WARN, eventType, message)
}

// Error logs an error message
func (r *Repository) Error(eventType, message string) error {
	return r.log(ERROR, eventType, message)
}

// Fatal logs a fatal message
func (r *Repository) Fatal(eventType, message string) error {
	return r.log(FATAL, eventType, message)
}

// --------------------------------------------
// Other funcs
// --------------------------------------------
func (r *Repository) GetInstrumentToken(exchange, symbol string) (uint32, error) {
	var instrument instrument.InstrumentModel
	err := r.DB.Where("exchange = ? AND tradingsymbol = ?", exchange, symbol).First(&instrument).Error
	if err != nil {
		return 0, err
	}
	return uint32(instrument.InstrumentToken), nil
}
