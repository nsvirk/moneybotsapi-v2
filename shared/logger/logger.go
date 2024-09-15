package logger

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nsvirk/moneybotsapi/shared/zaplogger"
	"gorm.io/datatypes"
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

// Log represents a log entry in the database
type Log struct {
	ID        uint       `gorm:"primaryKey"`
	Timestamp *time.Time `gorm:"index"`
	Level     *LogLevel  `gorm:"index"`
	Message   *string
	// Fields    *string // JSON string of fields
	Fields    datatypes.JSON `gorm:"type:jsonb"` // Changed to JSONB type
	tableName string         `gorm:"-"`          // This field is used internally and not stored in the database
}

// TableName overrides the table name used by Log
func (l *Log) TableName() string {
	return l.tableName
}

// Logger is the main struct for the logger
type Logger struct {
	db        *gorm.DB
	tableName string
}

// New creates a new Logger instance
func New(db *gorm.DB, tableName string) (*Logger, error) {
	logger := &Logger{
		db:        db,
		tableName: tableName,
	}
	if err := db.Table(tableName).AutoMigrate(&Log{}); err != nil {
		return nil, fmt.Errorf("failed to migrate Log for table %s: %v", tableName, err)
	}
	return logger, nil
}

// // log is a helper function to insert a log entry into the database
// func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) error {
// 	var fieldsJSON *string
// 	if len(fields) > 0 {
// 		jsonStr, err := json.Marshal(fields)
// 		if err != nil {
// 			return fmt.Errorf("failed to marshal fields: %v", err)
// 		}
// 		strJSON := string(jsonStr)
// 		fieldsJSON = &strJSON
// 	}

// 	timestamp := time.Now()
// 	entry := Log{
// 		Timestamp: &timestamp,
// 		Level:     &level,
// 		Message:   &message,
// 		Fields:    fieldsJSON,
// 		tableName: l.tableName,
// 	}

// 	if err := l.db.Table(l.tableName).Create(&entry).Error; err != nil {
// 		return fmt.Errorf("failed to insert log entry: %v", err)
// 	}

// 	return nil
// }

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
		Timestamp: &timestamp,
		Level:     &level,
		Message:   &message,
		Fields:    fieldsJSON,
		tableName: l.tableName,
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
