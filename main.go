// Package main is the entry point for the Moneybots API
package main

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/config"
	"github.com/nsvirk/moneybotsapi/database"
	"github.com/nsvirk/moneybotsapi/services"
	"github.com/nsvirk/moneybotsapi/shared/logger"
	"github.com/nsvirk/moneybotsapi/shared/middleware"
	"github.com/nsvirk/moneybotsapi/shared/zaplogger"
)

func main() {
	// Setup logger
	defer zaplogger.Sync()
	zaplogger.SetLogLevel("debug")

	// startUpMessage
	zaplogger.Info(config.SingleLine)
	zaplogger.Info("Moneybots API")

	// Load configuration
	cfg, err := config.Get()
	if err != nil {
		zaplogger.Fatal("failed to load configuration", zaplogger.Fields{"error": err})
	} else {
		zaplogger.Info("  * loaded")
		// zaplogger.Info(config.SingleLine)
	}

	// Print the configuration
	fmt.Println(cfg.String())

	// Create a new Echo instance
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Setup middleware
	middleware.SetupLoggerMiddleware(e)

	// Connect to Postgres
	db, err := database.ConnectPostgres(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}

	// Connect Redis
	redisClient, err := database.ConnectRedis(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize the logger - logs will be stored in the database
	logger, err := logger.New(db)
	if err != nil {
		panic(err)
	}

	// Use the logger
	err = logger.Info("Application started", map[string]interface{}{
		"name":    cfg.APIName,
		"version": cfg.APIVersion,
	})
	if err != nil {
		panic(err)
	}

	logger.Info("Application dummy", nil)

	// Setup routes
	setupRoutes(e, db, redisClient)

	// Setup and start cron jobs
	cronService := services.NewCronService(e, cfg, db, redisClient, logger)
	cronService.Start()

	// Start the server
	startServer(e, cfg)

}

// startServer starts the Echo server on the specified port
func startServer(e *echo.Echo, cfg *config.Config) {
	port := cfg.ServerPort
	if port == "" {
		port = "3007"
	}
	startupMessage := fmt.Sprintf("%s %s Server [:%s] started", cfg.APIName, cfg.APIVersion, cfg.ServerPort)

	zaplogger.Info(config.SingleLine)
	zaplogger.Info(startupMessage)
	zaplogger.Info(config.SingleLine)
	e.Logger.Infof(startupMessage)
	e.Logger.Fatal(e.Start(":" + port))
}
