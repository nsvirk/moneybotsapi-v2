package state

import (
	"time"
)

var StateTableName = "state"

type StateEntry struct {
	Key       string `gorm:"primaryKey"`
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (StateEntry) TableName() string {
	return StateTableName
}
