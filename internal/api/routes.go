// Package api contains the API routes for the Moneybots API
package api

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"

	"github.com/nsvirk/moneybotsapi/internal/api/handlers"
	"github.com/nsvirk/moneybotsapi/internal/api/middleware"
	"github.com/nsvirk/moneybotsapi/internal/config"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// SetupRoutes configures the routes for the API
func SetupRoutes(e *echo.Echo, db *gorm.DB, redisClient *redis.Client) {

	// Create a group for all API routes
	api := e.Group("/api")

	// Index route
	api.GET("/", indexRoute)

	// Session routes (unprotected)
	sessionService := service.NewSessionService(db)
	sessionHandler := handlers.NewSessionHandler(sessionService)
	sessionGroup := api.Group("/session")
	sessionGroup.GET("/login", sessionHandler.GenerateSession)
	sessionGroup.GET("/totp", sessionHandler.GenerateTOTP)
	sessionGroup.GET("/valid", sessionHandler.CheckSessionValid)

	// Instrument routes (protected)
	instrumentHandler := handlers.NewInstrumentHandler(db)
	instrumentGroup := api.Group("/instrument")
	instrumentGroup.Use(middleware.AuthMiddleware(db))
	instrumentGroup.GET("/query", instrumentHandler.QueryInstruments)
	instrumentGroup.GET("/tokens", instrumentHandler.GetInstrumentToTokenMap)
	instrumentGroup.GET("/symbols", instrumentHandler.GetTokensToInstrumentMap)

	// Optionchain routes (protected)
	optionchainGroup := api.Group("/instrument/oc")
	optionchainGroup.Use(middleware.AuthMiddleware(db))
	optionchainGroup.GET("", instrumentHandler.GetOptionChainInstruments)
	optionchainGroup.GET("/names", instrumentHandler.GetOptionChainNames)

	// Index routes (protected)
	indexGroup := api.Group("/index")
	indexGroup.Use(middleware.AuthMiddleware(db))
	indexGroup.GET("/names", instrumentHandler.GetIndexNames)
	indexGroup.GET("/instruments", instrumentHandler.GetIndexInstruments)

	// Ticker routes (protected)
	tickerService := service.NewTickerService(db, redisClient)
	tickerHandler := handlers.NewTickerHandler(tickerService)
	tickerGroup := api.Group("/ticker")
	tickerGroup.Use(middleware.AuthMiddleware(db))
	tickerGroup.GET("/instruments", tickerHandler.GetTickerInstruments)
	tickerGroup.POST("/instruments", tickerHandler.AddTickerInstruments)
	tickerGroup.DELETE("/instruments", tickerHandler.DeleteTickerInstruments)
	tickerGroup.GET("/start", tickerHandler.TickerStart)
	tickerGroup.GET("/stop", tickerHandler.TickerStop)
	tickerGroup.GET("/restart", tickerHandler.TickerRestart)
	tickerGroup.GET("/status", tickerHandler.TickerStatus)

	// Quote routes (protected)
	quoteService := service.NewQuoteService(db)
	quoteHandler := handlers.NewQuoteHandler(quoteService)
	quoteGroup := api.Group("/quote")
	quoteGroup.Use(middleware.AuthMiddleware(db))
	quoteGroup.GET("", quoteHandler.GetQuote)
	quoteGroup.GET("/ohlc", quoteHandler.GetOHLC)
	quoteGroup.GET("/ltp", quoteHandler.GetLTP)

	// Stream routes (protected)
	streamHandler := handlers.NewStreamHandler(db)
	streamGroup := api.Group("/stream")
	streamGroup.Use(middleware.AuthMiddleware(db))
	streamGroup.POST("/ticks", streamHandler.StreamTickerData)

}

// indexRoute sets up the index route for the API
func indexRoute(c echo.Context) error {
	cfg, err := config.Get()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	message := fmt.Sprintf("%s %s", cfg.APIName, cfg.APIVersion)
	return response.SuccessResponse(c, message)
}
