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
	"github.com/nsvirk/moneybotsapi/pkg/utils/logger"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"gorm.io/gorm"
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
	}
	zaplogger.Info(config.SingleLine)

	// Print the configuration
	fmt.Println(cfg.String())

	// Create a new Echo instance
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Setup middleware
	middleware.SetupLoggerMiddleware(e)

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

	// Setup routes
	api.SetupRoutes(e, db, redisClient)

	// Setup and start cron jobs
	cronService := service.NewCronService(e, cfg, db, redisClient)
	cronService.Start()

	// Setup and start ticks
	publishService := service.NewPublishService(db, redisClient, cfg.PostgresDsn)
	go publishService.PublishTicksToRedisChannel()

	// Start the server
	startServer(e, cfg, db)

}

// startServer starts the Echo server on the specified port
func startServer(e *echo.Echo, cfg *config.Config, db *gorm.DB) {
	// Initialize the logger - logs will be stored in the database
	logger, err := logger.New(db, "MAIN")
	if err != nil {
		panic(err)
	}

	port := cfg.ServerPort
	if port == "" {
		port = "3007"
	}

	// Database log
	logger.Info("Server started", map[string]interface{}{
		"name":    cfg.APIName,
		"version": cfg.APIVersion,
		"port":    port,
	})

	// Console log
	startupMessage := fmt.Sprintf("%s %s Server [:%s] started", cfg.APIName, cfg.APIVersion, cfg.ServerPort)
	zaplogger.Info(config.SingleLine)
	zaplogger.Info(startupMessage)
	zaplogger.Info(config.SingleLine)
	e.Logger.Infof(startupMessage)
	e.Logger.Fatal(e.Start(":" + port))
}
