// Package main is the entry point for the Moneybots API
package main

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/api"
	"github.com/nsvirk/moneybotsapi/internal/api/middleware"
	"github.com/nsvirk/moneybotsapi/internal/config"
	"github.com/nsvirk/moneybotsapi/internal/repository"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
)

func main() {
	// Load configuration
	cfg, err := config.Get()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Print the configuration
	fmt.Println(cfg.String())

	// Connect to Postgres
	db, err := repository.ConnectPostgres(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}

	// Connect Redis
	redisClient, err := repository.ConnectRedis(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Init logger
	err = zaplogger.InitLogger(db)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Setup logger
	defer zaplogger.Sync()
	zaplogger.SetLogLevel(cfg.ServerLogLevel)

	// startUpMessage
	zaplogger.Info(cfg.APIName + " - " + cfg.APIVersion + " initialized")
	zaplogger.Info("Postgres initialized")
	zaplogger.Info("Redis initialized")

	// Create a new Echo instance
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Setup middleware
	middleware.SetupLoggerMiddleware(e)

	// Setup routes
	api.SetupRoutes(e, cfg, db, redisClient)

	// Setup and start cron jobs
	cronService := service.NewCronService(e, cfg, db, redisClient)
	// start cron jobs
	cronService.Start()

	// Setup and start ticks
	publishService := service.NewPublishService(db, redisClient, cfg.PostgresDsn)
	go publishService.PublishTicksToRedisChannel()

	// Start the server
	startServer(e, cfg)

}

// startServer starts the Echo server on the specified port
func startServer(e *echo.Echo, cfg *config.Config) {
	port := cfg.ServerPort
	if port == "" {
		port = "3007"
	}
	zaplogger.Info("SERVER STARTED ON PORT " + port)
	e.Logger.Fatal(e.Start(":" + port))

}
