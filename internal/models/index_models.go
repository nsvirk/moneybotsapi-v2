// Package models contains the models for the Moneybots API
package models

import "time"

// TableName is the name of the table for instruments
var IndexTableName = "indices"

// Company Name	Industry	Symbol	Series	ISIN Code

// Index represents a trading index
type IndexModel struct {
	ID            uint32    `gorm:"primaryKey;autoIncrement" json:"-"`
	Index         string    `json:"index,omitempty" gorm:"index"`
	Exchange      string    `json:"exchange,omitempty"`
	Tradingsymbol string    `json:"tradingsymbol,omitempty" gorm:"index"`
	CompanyName   string    `json:"company_name,omitempty"`
	Industry      string    `json:"industry,omitempty" gorm:"index"`
	Series        string    `json:"series,omitempty"`
	ISINCode      string    `json:"isin_code,omitempty"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"-"`
}

// TableName specifies the table name for the Index model
func (IndexModel) TableName() string {
	return IndexTableName
}
