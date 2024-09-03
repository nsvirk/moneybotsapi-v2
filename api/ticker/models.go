package ticker

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
)

const (
	TickerInstrumentsTableName = "ticker_instruments"
	TickerDataTableName        = "ticker_data"
	TickerLogTableName         = "ticker_logs"
)

// TICKER INSTRUMENTS -------------------------------------------------
// TickerInstrument represents the instruments for which tick data is subscribed
type TickerInstrument struct {
	UserID          string    `gorm:"uniqueIndex:idx_userId_instrument,priority:1;type:varchar(10)" json:"user_id"`
	Instrument      string    `gorm:"uniqueIndex:idx_userId_instrument,priority:2" json:"instrument"`
	InstrumentToken uint32    `json:"instrument_token"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TickerInstrument) TableName() string {
	return TickerInstrumentsTableName
}

// TICKER DATA --------------------------------------------------------
// TickerData represents the tick data for an instrument
type TickerData struct {
	Instrument         string    `gorm:"index" json:"instrument"`
	Mode               string    `gorm:"type:varchar(10)" json:"mode"`
	InstrumentToken    uint32    `gorm:"primaryKey"  json:"instrument_token"`
	IsTradable         bool      `json:"is_tradable"`
	IsIndex            bool      `json:"is_index"`
	Timestamp          time.Time `json:"timestamp"`
	LastTradeTime      time.Time `json:"last_trade_time"`
	LastPrice          float64   `gorm:"type:decimal(10,2);column:last_price" json:"last_price"`
	LastTradedQuantity uint32    `gorm:"type:bigint;column:last_traded_quantity" json:"last_traded_quantity"`
	TotalBuyQuantity   uint32    `gorm:"type:bigint;column:total_buy_quantity" json:"total_buy_quantity"`
	TotalSellQuantity  uint32    `gorm:"type:bigint;column:total_sell_quantity" json:"total_sell_quantity"`
	VolumeTraded       uint32    `gorm:"type:bigint;column:volume" json:"volume"`
	// TotalBuy           uint32         `gorm:"type:bigint" json:"total_buy"`
	// TotalSell          uint32         `gorm:"type:bigint" json:"total_sell"`
	AverageTradePrice float64        `gorm:"type:decimal(10,2);column:average_price" json:"average_price"`
	OI                uint32         `gorm:"type:bigint;column:oi" json:"oi"`
	OIDayHigh         uint32         `gorm:"type:bigint;column:oi_day_high" json:"oi_day_high"`
	OIDayLow          uint32         `gorm:"type:bigint;column:oi_day_low" json:"oi_day_low"`
	NetChange         float64        `gorm:"type:decimal(10,2)" json:"net_change"`
	OHLC              datatypes.JSON `gorm:"type:jsonb;column:ohlc" json:"ohlc"`
	Depth             datatypes.JSON `gorm:"type:jsonb;column:depth" json:"depth"`
	UpdatedAt         time.Time      `gorm:"autoUpdateTime:nano"  json:"updated_at"`
}

type TickerDataOHLC struct {
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

type TickerDataDepth struct {
	Buy  [5]TickerDataDepthItem `json:"buy"`
	Sell [5]TickerDataDepthItem `json:"sell"`
}

type TickerDataDepthItem struct {
	Price    float64 `json:"price"`
	Quantity uint32  `json:"quantity"`
	Orders   uint32  `json:"orders"`
}

func (t *TickerData) GetOHLC() (TickerDataOHLC, error) {
	var ohlc TickerDataOHLC
	err := json.Unmarshal(t.OHLC, &ohlc)
	return ohlc, err
}

func (t *TickerData) GetDepth() (TickerDataDepth, error) {
	var depth TickerDataDepth
	err := json.Unmarshal(t.Depth, &depth)
	return depth, err
}

func (TickerData) TableName() string {
	return TickerDataTableName
}

func (o *TickerDataOHLC) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, &o)
}

func (o TickerDataOHLC) Value() (driver.Value, error) {
	return json.Marshal(o)
}

// TICKER LOGS -----------------------------------------------------
// LogLevel represents the severity of a log message
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
)

type TickerLog struct {
	ID        uint32     `gorm:"primaryKey"`
	Timestamp *time.Time `gorm:"index"`
	Level     *LogLevel
	EventType *string
	Message   *string
}

func (TickerLog) TableName() string {
	return TickerLogTableName
}
