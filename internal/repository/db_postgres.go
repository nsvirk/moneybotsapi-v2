// Package repository contains the repository layer for the Moneybots API
package repository

import (
	"fmt"

	"github.com/nsvirk/moneybotsapi/internal/config"
	"github.com/nsvirk/moneybotsapi/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectPostgres connects to a Postgres database and returns a GORM database object
func ConnectPostgres(cfg *config.Config) (*gorm.DB, error) {
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
	postgresDSN := cfg.PostgresDsn + " search_path=api,public"
	db, err := gorm.Open(postgres.Open(postgresDSN), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Postgres: %v", err)
	}

	// Create the schema if it doesn't exist
	createSchemaSql := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", cfg.PostgresSchema)
	if err := db.Exec(createSchemaSql).Error; err != nil {
		panic("failed to create schema: " + err.Error())
	}

	// AutoMigrate will create tables and add/modify columns
	if err := autoMigrate(db, cfg); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %v", err)
	}

	// Set the ticker data table as unlogged
	err = setTickerDataTableAsUnlogged(db, cfg)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func autoMigrate(db *gorm.DB, cfg *config.Config) error {
	tables := []struct {
		name  string
		model interface{}
	}{
		{models.SessionsTableName, &models.SessionModel{}},
		{models.InstrumentsTableName, &models.InstrumentModel{}},
		{models.IndexTableName, &models.IndexModel{}},
		{models.TickerInstrumentsTableName, &models.TickerInstrument{}},
		{models.TickerLogTableName, &models.TickerLog{}},
		{models.TickerDataTableName, &models.TickerData{}},
	}

	for _, table := range tables {
		err := db.Table(cfg.PostgresSchema + "." + table.name).AutoMigrate(&table.model)
		if err != nil {
			return fmt.Errorf("failed to auto migrate table: %s, err:%v", table.name, err)
		}
	}
	return nil
}

func setTickerDataTableAsUnlogged(db *gorm.DB, cfg *config.Config) error {
	// Set the table as unlogged
	table := models.TickerDataTableName
	if err := db.Table(cfg.PostgresSchema + "." + table).Exec("ALTER TABLE " + table + " SET UNLOGGED").Error; err != nil {
		return fmt.Errorf("failed to set table as unlogged: %v", err)
	}
	return nil
}
