// Package zaplogger contains utility functions and types
package zaplogger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
)

var log *zap.Logger
var zapConfig zap.Config

// Fields type, used to pass to `WithFields`.
type Fields map[string]interface{}

// LogModel represents the structure of the log entry in the database
type LogModel struct {
	ID        uint      `gorm:"primaryKey"`
	Timestamp time.Time `gorm:"index"`
	Level     string
	Caller    string
	Message   string
	Fields    string // JSON string of additional fields
}

// TableName specifies the table name for LogEntry
func (LogModel) TableName() string {
	return "_app_logs"
}

// DbWriter implements zapcore.WriteSyncer interface for database logging using GORM
type DbWriter struct {
	db *gorm.DB
}

// LogData represents the structure of the JSON log data// LogData represents the structure of the JSON log data
type LogData struct {
	Level     string                 `json:"level"`     // Level
	Timestamp string                 `json:"timestamp"` // Timestamp
	Caller    string                 `json:"caller"`    // Caller
	Message   string                 `json:"message"`   // Message
	Fields    map[string]interface{} `json:"fields"`    // Additional fields
}

func (w *DbWriter) Write(p []byte) (n int, err error) {
	var logData LogData
	err = json.Unmarshal(p, &logData)
	if err != nil {
		return 0, err
	}

	// Extract additional fields
	var rawMessage map[string]json.RawMessage
	err = json.Unmarshal(p, &rawMessage)
	if err != nil {
		return 0, err
	}

	additionalFields := make(map[string]interface{})
	for k, v := range rawMessage {
		if k != "level" && k != "timestamp" && k != "caller" && k != "message" {
			additionalFields[k] = v
		}
	}

	fieldsJSON, err := json.Marshal(additionalFields)
	if err != nil {
		return 0, err
	}

	timestamp, err := time.Parse("2006-01-02T15:04:05.999-0700", logData.Timestamp)
	if err != nil {
		return 0, err
	}

	logRecord := LogModel{
		Timestamp: timestamp,
		Level:     logData.Level,
		Caller:    logData.Caller,
		Message:   logData.Message,
		Fields:    string(fieldsJSON), // Store only the additional fields
	}

	result := w.db.Create(&logRecord)
	if result.Error != nil {
		return 0, result.Error
	}
	return len(p), nil
}

func (w *DbWriter) Sync() error {
	return nil
}

// func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
// 	enc.AppendString(t.Format("2006-01-02 15:04:05"))
// }

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02T15:04:05.999-0700"))
}

func init() {
	zapConfig = zap.Config{
		Encoding:         "console",
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:   "message",
			LevelKey:     "level",
			TimeKey:      "timestamp",
			CallerKey:    "caller",
			EncodeLevel:  zapcore.CapitalLevelEncoder,
			EncodeTime:   customTimeEncoder,
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}

	var err error
	log, err = zapConfig.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
}

// InitLogger initializes the logger with both console and database output
func InitLogger(db *gorm.DB) error {

	// Create the table if it doesn't exist
	err := db.AutoMigrate(&LogModel{})
	if err != nil {
		return fmt.Errorf("failed to auto migrate: %v", err)
	}

	// Create DbWriter
	dbWriter := &DbWriter{db: db}

	// Create encoders
	consoleEncoder := zapcore.NewConsoleEncoder(zapConfig.EncoderConfig)
	dbEncoder := zapcore.NewJSONEncoder(zapConfig.EncoderConfig)

	// Create core
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapConfig.Level),
		zapcore.NewCore(dbEncoder, zapcore.AddSync(dbWriter), zapConfig.Level),
	)

	log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return nil
}

// SetLogLevel sets the logging level
func SetLogLevel(level string) {
	var l zapcore.Level
	switch level {
	case "debug":
		l = zapcore.DebugLevel
	case "info":
		l = zapcore.InfoLevel
	case "warn":
		l = zapcore.WarnLevel
	case "error":
		l = zapcore.ErrorLevel
	default:
		l = zapcore.InfoLevel
	}
	log.Core().Enabled(l)
}

// Info logs an info message
func Info(msg string, fields ...Fields) {
	if len(fields) > 0 {
		log.Info(msg, getZapFields(fields[0])...)
	} else {
		log.Info(msg)
	}
}

// Debug logs a debug message
func Debug(msg string, fields ...Fields) {
	if len(fields) > 0 {
		log.Debug(msg, getZapFields(fields[0])...)
	} else {
		log.Debug(msg)
	}
}

// Warn logs a warning message
func Warn(msg string, fields ...Fields) {
	if len(fields) > 0 {
		log.Warn(msg, getZapFields(fields[0])...)
	} else {
		log.Warn(msg)
	}
}

// Error logs an error message
func Error(msg string, fields ...Fields) {
	if len(fields) > 0 {
		log.Error(msg, getZapFields(fields[0])...)
	} else {
		log.Error(msg)
	}
}

// Fatal logs a fatal message and exits the program
func Fatal(msg string, fields ...Fields) {
	if len(fields) > 0 {
		log.Fatal(msg, getZapFields(fields[0])...)
	} else {
		log.Fatal(msg)
	}
}

// WithFields adds fields to the logger
func WithFields(fields Fields) *zap.Logger {
	return log.With(getZapFields(fields)...)
}

// TimeTrack logs the time taken for a function to execute
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	Info(name+" took "+elapsed.String(), Fields{"duration": elapsed})
}

// getZapFields converts our Fields type to zap.Field slice
func getZapFields(fields Fields) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return zapFields
}

// Sync flushes any buffered log entries
func Sync() error {
	return log.Sync()
}
