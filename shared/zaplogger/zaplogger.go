package zaplogger

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// Fields type, used to pass to `WithFields`.
type Fields map[string]interface{}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05"))
}

func init() {
	config := zap.Config{
		Encoding:         "console",
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:   "message",
			LevelKey:     "level",
			TimeKey:      "time",
			CallerKey:    "caller",
			EncodeLevel:  zapcore.CapitalColorLevelEncoder,
			EncodeTime:   customTimeEncoder,
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}

	var err error
	log, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
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

// Usage:
// package main

// import (
// 	"time"
// 	"your_project/logger"
// )

// func main() {
// 	defer logger.Sync()

// 	logger.SetLogLevel("debug")

// 	logger.Info("Application started")
// 	logger.Debug("This is a debug message")
// 	logger.Warn("This is a warning", logger.Fields{"code": 123})

// 	logger.Info("User logged in", logger.Fields{
// 		"userId":    1001,
// 		"username":  "john_doe",
// 		"loginTime": time.Now().Format(time.RFC3339),
// 	})

// 	contextLogger := logger.WithFields(logger.Fields{
// 		"component": "user_service",
// 		"version":   "1.0.0",
// 	})
// 	contextLogger.Info("Processing user data")

// 	defer logger.TimeTrack(time.Now(), "LongOperation")
// 	// Your long operation here
// 	time.Sleep(2 * time.Second)
// }
