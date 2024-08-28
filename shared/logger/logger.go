package logger

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// LogLevel represents the severity of a log message
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
)

// APILog represents a log entry in the database
type APILog struct {
	ID        uint       `gorm:"primaryKey"`
	Timestamp *time.Time `gorm:"index"`
	Level     *LogLevel
	Message   *string
	Fields    *string // JSON string of fields
}

// TableName overrides the table name used by APILog
func (APILog) TableName() string {
	return "api_logs"
}

// Logger is the main struct for the dblogger
type Logger struct {
	db *gorm.DB
}

// New creates a new Logger instance
func New(db *gorm.DB) (*Logger, error) {
	if err := db.AutoMigrate(&APILog{}); err != nil {
		return nil, fmt.Errorf("failed to migrate APILog: %v", err)
	}
	return &Logger{db: db}, nil
}

// log is a helper function to insert a log entry into the database
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) error {
	var fieldsJSON *string
	if len(fields) > 0 {
		jsonStr, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("failed to marshal fields: %v", err)
		}
		strJSON := string(jsonStr)
		fieldsJSON = &strJSON
	}

	timestamp := time.Now()
	entry := APILog{
		Timestamp: &timestamp,
		Level:     &level,
		Message:   &message,
		Fields:    fieldsJSON,
	}

	if err := l.db.Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to insert log entry: %v", err)
	}

	return nil
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields map[string]interface{}) error {
	return l.log(DEBUG, message, fields)
}

// Info logs an info message
func (l *Logger) Info(message string, fields map[string]interface{}) error {
	return l.log(INFO, message, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields map[string]interface{}) error {
	return l.log(WARN, message, fields)
}

// Error logs an error message
func (l *Logger) Error(message string, fields map[string]interface{}) error {
	return l.log(ERROR, message, fields)
}

// Fatal logs a fatal message
func (l *Logger) Fatal(message string, fields map[string]interface{}) error {
	return l.log(FATAL, message, fields)
}
