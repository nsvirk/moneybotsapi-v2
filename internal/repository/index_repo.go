// Package repository contains the repository layer for the Moneybots API
package repository

import (
	"fmt"

	"github.com/nsvirk/moneybotsapi/internal/models"
	"gorm.io/gorm"
)

// Repository is the database repository for indices
type IndexRepository struct {
	DB *gorm.DB
}

// NewIndexRepository creates a new index repository
func NewIndexRepository(db *gorm.DB) *IndexRepository {
	return &IndexRepository{DB: db}
}

// TruncateIndicesTable truncates the indices table
func (r *IndexRepository) TruncateIndicesTable() error {
	return r.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", models.IndexTableName)).Error
}

// InsertIndices inserts a batch of indices into the database
func (r *IndexRepository) InsertIndices(indexInstruments []models.IndexModel) (int64, error) {
	// insert the records into the database
	result := r.DB.Create(indexInstruments)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to insert batch into %s: %v", models.IndexTableName, result.Error)
	}
	return result.RowsAffected, nil
}

// GetIndicesRecordCount returns the number of records in the indices table
func (r *IndexRepository) GetIndicesRecordCount() (int64, error) {
	var count int64
	err := r.DB.Table(models.IndexTableName).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get indices record count: %v", err)
	}
	return count, nil
}

// GetAllIndicesNames gets the names of all indices
func (r *IndexRepository) GetAllIndicesNames() ([]models.IndexModel, error) {
	var indices []models.IndexModel
	err := r.DB.Table(models.IndexTableName).
		Select("DISTINCT exchange, index").
		Find(&indices).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch distinct indices: %v", err)
	}
	return indices, nil
}

// GetAllDistinctIndexSymbol gets all distinct indices
func (r *IndexRepository) GetAllDistinctIndexSymbol() ([]models.IndexModel, error) {
	var indices []models.IndexModel
	err := r.DB.Table(models.IndexTableName).
		Select("DISTINCT exchange, tradingsymbol").
		Find(&indices).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch distinct indices: %v", err)
	}
	return indices, nil
}

// GetIndices gets the names of all indices for a given exchange
func (r *IndexRepository) GetIndices(exchange string) ([]models.IndexModel, error) {
	var indices []models.IndexModel
	err := r.DB.Table(models.IndexTableName).
		Where("exchange = ?", exchange).
		Find(&indices).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch indices: %v", err)
	}
	return indices, nil
}

// GetIndicesNames gets the names of all indices for a given exchange
func (r *IndexRepository) GetIndicesNames(exchange string) ([]string, error) {
	var indices []string
	err := r.DB.Table(models.IndexTableName).
		Select("DISTINCT index").
		Where("exchange = ?", exchange).
		Find(&indices).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch `%s` index names: %v", exchange, err)
	}
	return indices, nil
}

// GetIndexInstruments fetches the instruments for a given index
func (r *IndexRepository) GetIndexInstruments(exchange, index string) ([]models.IndexModel, error) {
	var indexInstruments []models.IndexModel
	err := r.DB.Where("index = ?", index).
		Where("exchange = ?", exchange).
		Find(&indexInstruments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch `%s` `%s` instruments: %v", exchange, index, err)
	}

	return indexInstruments, nil
}
