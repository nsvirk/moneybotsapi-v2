// Package logger contains utility functions and types
package logger

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var LogsTableName = "_logs"

// LogLevel represents the severity of a log message
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
)

// Log represents a log entry in the database
type Log struct {
	ID        uint32    `gorm:"primaryKey"`
	Timestamp time.Time `gorm:"index"`
	Package   string    `gorm:"index"`
	Level     LogLevel  `gorm:"index"`
	Message   string
	Fields    datatypes.JSON `gorm:"type:jsonb"` // Changed to JSONB type
}

// TableName overrides the table name used by Log
func (l *Log) TableName() string {
	return LogsTableName
}

// Logger is the main struct for the logger
type Logger struct {
	db          *gorm.DB
	packageName string
	tableName   string
}

// New creates a new Logger instance
func New(db *gorm.DB, packageName string) (*Logger, error) {
	logger := &Logger{
		db:          db,
		packageName: packageName,
	}
	if err := db.Table(LogsTableName).AutoMigrate(&Log{}); err != nil {
		return nil, fmt.Errorf("failed to migrate Log for table %s: %v", LogsTableName, err)
	}
	return logger, nil
}

// log is a helper function to insert a log entry into the database
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) error {
	var fieldsJSON datatypes.JSON
	if len(fields) > 0 {
		jsonBytes, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("failed to marshal fields: %v", err)
		}
		fieldsJSON = datatypes.JSON(jsonBytes)
	}

	timestamp := time.Now()
	entry := Log{
		Timestamp: timestamp,
		Package:   l.packageName,
		Level:     level,
		Message:   message,
		Fields:    fieldsJSON,
	}

	if err := l.db.Table(l.tableName).Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to insert log entry: %v", err)
	}

	return nil
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields map[string]interface{}) {
	err := l.log(DEBUG, message, fields)
	if err != nil {
		zaplogger.Error("Failed to log DEBUG message", map[string]interface{}{
			"error": err,
		})
	}
}

// Info logs an info message
func (l *Logger) Info(message string, fields map[string]interface{}) {
	err := l.log(INFO, message, fields)
	if err != nil {
		zaplogger.Error("Failed to log INFO message", map[string]interface{}{
			"error": err,
		})
	}
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields map[string]interface{}) {
	err := l.log(WARN, message, fields)
	if err != nil {
		zaplogger.Error("Failed to log WARN message", map[string]interface{}{
			"error": err,
		})
	}
}

// Error logs an error message
func (l *Logger) Error(message string, fields map[string]interface{}) {
	err := l.log(ERROR, message, fields)
	if err != nil {
		zaplogger.Error("Failed to log ERROR message", map[string]interface{}{
			"error": err,
		})
	}
}

// Fatal logs a fatal message
func (l *Logger) Fatal(message string, fields map[string]interface{}) {
	err := l.log(FATAL, message, fields)
	if err != nil {
		zaplogger.Error("Failed to log FATAL message", map[string]interface{}{
			"error": err,
		})
	}
}
