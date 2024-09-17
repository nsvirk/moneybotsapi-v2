// Package index manages the Index instruments
// repository.go - Database operations and data access
package index

import (
	"fmt"

	"gorm.io/gorm"
)

// Repository is the database repository for instruments
type Repository struct {
	DB *gorm.DB
}

// NewIndexRepository creates a new index repository
func NewIndexRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// TruncateIndices truncates the indices table
func (r *Repository) TruncateIndices() error {
	return r.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", IndexTableName)).Error
}

// InsertIndices inserts a batch of indices into the database
func (r *Repository) InsertIndices(indexInstruments []IndexModel) (int64, error) {
	// upsert the records into the database
	result := r.DB.Create(indexInstruments)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to insert batch into %s: %v", IndexTableName, result.Error)
	}
	return result.RowsAffected, nil
}

// GetNSEIndexInstruments fetches the instruments for a given NSE index
func (r *Repository) GetNSEIndexInstruments(indexName string) ([]IndexModel, error) {
	var indexInstruments []IndexModel
	err := r.DB.Where("index_name = ?", indexName).Find(&indexInstruments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index instruments: %v", err)
	}

	return indexInstruments, nil
}
