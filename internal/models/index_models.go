// Package models contains the models for the Moneybots API
package models

import "time"

// TableName is the name of the table for instruments
var IndexTableName = "indices"

// Company Name	Industry	Symbol	Series	ISIN Code

// Index represents a trading index
type IndexModel struct {
	ID            uint32    `gorm:"primaryKey;autoIncrement" json:"-"`
	Index         string    `json:"index" gorm:"index"`
	Exchange      string    `json:"exchange"`
	Tradingsymbol string    `json:"tradingsymbol" gorm:"index"`
	CompanyName   string    `json:"company_name"`
	Industry      string    `json:"industry" gorm:"index"`
	Series        string    `json:"series"`
	ISINCode      string    `json:"isin_code"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"-"`
}

// TableName specifies the table name for the Index model
func (IndexModel) TableName() string {
	return IndexTableName
}
