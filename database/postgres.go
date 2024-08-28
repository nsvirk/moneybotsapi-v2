// File: database/postgres.go

package database

import (
	"fmt"

	"github.com/nsvirk/moneybotsapi/api/instrument"
	"github.com/nsvirk/moneybotsapi/api/session"
	"github.com/nsvirk/moneybotsapi/api/ticker"
	"github.com/nsvirk/moneybotsapi/config"
	"github.com/nsvirk/moneybotsapi/shared/zaplogger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectPostgres connects to a Postgres database and returns a GORM database object
func ConnectPostgres(cfg *config.Config) (*gorm.DB, error) {
	zaplogger.Info(config.SingleLine)
	zaplogger.Info("Initializing Postgres")

	// Set up GORM logger
	var logLevel logger.LogLevel
	switch cfg.PostgresLogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	default:
		logLevel = logger.Info // Default to Info level
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	// Open database connection
	db, err := gorm.Open(postgres.Open(cfg.PostgresDsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Postgres: %v", err)
	}
	zaplogger.Info("  * connected")
	zaplogger.Info("  * checking tables")

	// AutoMigrate will create tables and add/modify columns
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %v", err)
	}

	// Verify that the tables are created
	if err := verifyTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&session.SessionModel{},
		&instrument.InstrumentModel{},
		&ticker.TickerInstrument{},
		&ticker.TickerLog{},
		&ticker.TickerData{},
	)
}

func verifyTables(db *gorm.DB) error {
	tables := []struct {
		name  string
		model interface{}
	}{
		{session.SessionsTableName, &session.SessionModel{}},
		{instrument.InstrumentsTableName, &instrument.InstrumentModel{}},
		{ticker.TickerInstrumentsTableName, &ticker.TickerInstrument{}},
		{ticker.TickerLogTableName, &ticker.TickerLog{}},
		{ticker.TickerDataTableName, &ticker.TickerData{}},
	}

	for _, table := range tables {
		if db.Migrator().HasTable(table.model) {
			zaplogger.Info("    - " + table.name + " \u2714")
		} else {
			return fmt.Errorf("failed to create table: " + table.name)
		}
	}

	return nil
}
