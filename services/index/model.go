// Package index manages the Index instruments
// model.go - Data models and structures
package index

import "time"

// TableName is the name of the table for instruments
var IndexTableName = "indices"

// Index represents a trading index
type IndexModel struct {
	ID         uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	IndexName  string    `json:"index_name" gorm:"uniqueIndex:idx_index_instrument"`
	Instrument string    `json:"instrument" gorm:"uniqueIndex:idx_index_instrument"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName specifies the table name for the Index model
func (IndexModel) TableName() string {
	return IndexTableName
}
