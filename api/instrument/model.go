// File: github.com/nsvirk/moneybotsapi/instrument/model.go

package instrument

import "time"

// TableName is the name of the table for instruments
var InstrumentsTableName = "api_instruments"

// Instrument represents a trading instrument
type InstrumentModel struct {
	InstrumentToken uint      `gorm:"primaryKey;uniqueIndex;index" csv:"instrument_token" json:"instrument_token"`
	ExchangeToken   uint      `csv:"exchange_token" json:"exchange_token"`
	Tradingsymbol   string    `gorm:"index:idx_exchange_tradingsymbol,priority:2;index:idx_exch_trading_expiry,priority:2;index:idx_exch_trading_expiry_strike,priority:2" csv:"tradingsymbol" json:"tradingsymbol"`
	Name            string    `csv:"name" json:"name"`
	LastPrice       float64   `csv:"last_price" json:"last_price"`
	Expiry          string    `gorm:"index:idx_exch_trading_expiry,priority:3;index:idx_exch_trading_expiry_strike,priority:3" csv:"expiry" json:"expiry"`
	Strike          float64   `gorm:"index:idx_exch_trading_expiry_strike,priority:4" csv:"strike" json:"strike"`
	TickSize        float64   `csv:"tick_size" json:"tick_size"`
	LotSize         uint      `csv:"lot_size" json:"lot_size"`
	InstrumentType  string    `csv:"instrument_type" json:"instrument_type"`
	Segment         string    `csv:"segment" json:"segment"`
	Exchange        string    `gorm:"index:idx_exchange_tradingsymbol,priority:1;index:idx_exch_trading_expiry,priority:1;index:idx_exch_trading_expiry_strike,priority:1" csv:"exchange" json:"exchange"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName specifies the table name for the Instrument model
func (InstrumentModel) TableName() string {
	return InstrumentsTableName
}
