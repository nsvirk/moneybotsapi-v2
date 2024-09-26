// Package models contains the models for the Moneybots API
package models

import "time"

// TableName is the name of the table for instruments
var IndexTableName = "indices"

// Company Name	Industry	Symbol	Series	ISIN Code

// Index represents a trading index
type IndexModel struct {
	ID          uint32    `gorm:"primaryKey;autoIncrement" json:"-"`
	Index       string    `json:"index" gorm:"uniqueIndex:idx_index_instrument"`
	Instrument  string    `json:"instrument" gorm:"uniqueIndex:idx_index_instrument"`
	CompanyName string    `json:"company_name"`
	Industry    string    `json:"industry" gorm:"index"`
	Series      string    `json:"series"`
	ISINCode    string    `json:"isin_code"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"-"`
}

// TableName specifies the table name for the Index model
func (IndexModel) TableName() string {
	return IndexTableName
}
