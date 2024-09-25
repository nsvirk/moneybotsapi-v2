package state

import (
	"errors"

	"gorm.io/gorm"
)

type State struct {
	db *gorm.DB
}

func NewState(db *gorm.DB) (*State, error) {

	err := db.AutoMigrate(&StateEntry{})
	if err != nil {
		return nil, err
	}

	return &State{db: db}, nil
}

func (s *State) Get(key string) (string, error) {
	var entry StateEntry
	result := s.db.Where("key = ?", key).First(&entry)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return "", nil // Key not found, return empty string
		}
		return "", result.Error
	}
	return entry.Value, nil
}

func (s *State) Set(key, value string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var entry StateEntry
		result := tx.Where("key = ?", key).First(&entry)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// Create new entry if not found
				entry = StateEntry{Key: key, Value: value}
				return tx.Create(&entry).Error
			}
			return result.Error
		}
		// Update existing entry
		entry.Value = value
		return tx.Save(&entry).Error
	})
}

func (s *State) Delete(key string) error {
	return s.db.Where("key = ?", key).Delete(&StateEntry{}).Error
}
