// Package main is the entry point for the Moneybots API
package main

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/config"
	"github.com/nsvirk/moneybotsapi/database"
	"github.com/nsvirk/moneybotsapi/services"
	"github.com/nsvirk/moneybotsapi/shared/applogger"
	"github.com/nsvirk/moneybotsapi/shared/middleware"
)

func main() {
	// Setup logger
	defer applogger.Sync()
	applogger.SetLogLevel("debug")

	// startUpMessage
	applogger.Info(config.SingleLine)
	applogger.Info("Moneybots API")

	// Load configuration
	cfg, err := config.Get()
	if err != nil {
		applogger.Fatal("failed to load configuration", applogger.Fields{"error": err})
	} else {
		applogger.Info("  * loaded")
		// applogger.Info(config.SingleLine)
	}

	// fmt.Println(cfg.String())

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

	// Setup routes
	setupRoutes(e, db, redisClient)

	// Setup and start cron jobs
	cronService := services.NewCronService(e, cfg, db, redisClient)
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

	applogger.Info(config.SingleLine)
	applogger.Info(startupMessage)
	applogger.Info(config.SingleLine)
	e.Logger.Infof(startupMessage)
	e.Logger.Fatal(e.Start(":" + port))
}
