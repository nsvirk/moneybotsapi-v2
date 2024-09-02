package ticker

import (
	"fmt"
	"strconv"
	"time"

	"github.com/nsvirk/moneybotsapi/api/instrument"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// --------------------------------------------
// TickerInstruments func's grouped together
// --------------------------------------------
func (r *Repository) TruncateTickerInstruments() error {
	result := r.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", TickerInstrumentsTableName))
	if result.Error != nil {
		return fmt.Errorf("failed to truncate table %s: %v", TickerInstrumentsTableName, result.Error)
	}
	return nil
}

// UpsertQueriedInstruments upserts instruments queried from the instrument table
//
//	used by cron job to keep ticker instruments updated
func (r *Repository) UpsertQueriedInstruments(userID, exchange, tradingsymbol, expiry, strike, segment string) (map[string]interface{}, error) {
	query := r.DB.Model(&instrument.InstrumentModel{})

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

	var instruments []instrument.InstrumentModel
	if err := query.Find(&instruments).Error; err != nil {
		return nil, err
	}

	instrumentTokens := make(map[string]uint32)
	for _, instrument := range instruments {
		key := fmt.Sprintf("%s:%s", instrument.Exchange, instrument.Tradingsymbol)
		instrumentTokens[key] = uint32(instrument.InstrumentToken)
	}

	addedCount, updatedCount, err := r.upsertInstruments(userID, instrumentTokens)
	if err != nil {
		return nil, err
	}

	// var tickerInstruments []TickerInstrument
	// for _, instrument := range instruments {
	// 	tickerInstruments = append(tickerInstruments, TickerInstrument{
	// 		UserID:          userID,
	// 		Instrument:      fmt.Sprintf("%s:%s", instrument.Exchange, instrument.Tradingsymbol),
	// 		InstrumentToken: uint32(instrument.InstrumentToken),
	// 		UpdatedAt:       time.Now(),
	// 	})

	// addedCount, updatedCount, err := r.UpsertTickerInstruments(userID, instrumentTokens)
	// if err != nil {
	// 	return nil, err
	// }

	totalCount, err := r.GetTickerInstrumentCount(userID)
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"user_id": userID,
		"queried": len(instruments),
		"added":   addedCount,
		"updated": updatedCount,
		"total":   totalCount,
	}

	return response, nil
}

func (r *Repository) upsertInstruments(userID string, instrumentTokens map[string]uint32) (int, int, error) {
	addedCount := 0
	updatedCount := 0

	for instrument, token := range instrumentTokens {
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
			addedCount++
		} else {
			updatedCount++
		}
	}

	return addedCount, updatedCount, nil
}

func (r *Repository) AddTickerInstruments(userID string, tickerInstruments []TickerInstrument) (int64, error) {

	var upsertedCount int64

	for _, instrument := range tickerInstruments {
		result := r.DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "instrument"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"instrument_token", "updated_at"}),
		}).Create(&TickerInstrument{
			UserID:          userID,
			Instrument:      instrument.Instrument,
			InstrumentToken: instrument.InstrumentToken,
			UpdatedAt:       time.Now(),
		})

		if result.Error != nil {
			return upsertedCount, fmt.Errorf("error upserting instrument: %v", result.Error)
		}

		upsertedCount = result.RowsAffected
	}

	return upsertedCount, nil
}

func (r *Repository) GetTickerInstruments(userID string) ([]TickerInstrument, error) {
	var tickerInstruments []TickerInstrument
	err := r.DB.Where("user_id = ?", userID).Find(&tickerInstruments).Error
	return tickerInstruments, err
}

func (r *Repository) GetTickerInstrumentCount(userID string) (int64, error) {
	var count int64
	err := r.DB.Model(&TickerInstrument{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *Repository) DeleteTickerInstruments(userID string, instruments []string) (int64, error) {
	result := r.DB.Where("user_id = ? AND instrument IN ?", userID, instruments).Delete(&TickerInstrument{})
	return result.RowsAffected, result.Error
}

// --------------------------------------------
// TickerData func's grouped together
// --------------------------------------------
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

	err := r.DB.Transaction(func(tx *gorm.DB) error {
		for _, data := range uniqueTickerData {
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "instrument_token"}},
				DoUpdates: clause.AssignmentColumns([]string{"timestamp", "last_trade_time", "last_price", "last_traded_quantity", "total_buy_quantity", "total_sell_quantity", "volume", "average_price", "oi", "oi_day_high", "oi_day_low", "net_change", "ohlc", "depth", "updated_at"}),
			}).Create(&data)

			if result.Error != nil {
				return fmt.Errorf("failed to upsert ticker data for instrument %d: %v", data.InstrumentToken, result.Error)
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
