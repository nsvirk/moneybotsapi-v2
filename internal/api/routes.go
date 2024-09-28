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
	sessionGroup.POST("/login", sessionHandler.GenerateSession)
	sessionGroup.POST("/totp", sessionHandler.GenerateTOTP)

	// Instrument routes (protected)
	instrumentHandler := handlers.NewInstrumentHandler(db)
	instrumentGroup := api.Group("/instruments")
	instrumentGroup.Use(middleware.AuthMiddleware(db))
	instrumentGroup.GET("/query", instrumentHandler.QueryInstruments)
	instrumentGroup.GET("/exchange_names/:expiry", instrumentHandler.GetExchangeNames)
	instrumentGroup.GET("/instrument_tokens", instrumentHandler.GetInstrumentToTokenMap)
	instrumentGroup.GET("/symbols", instrumentHandler.GetTokensToInstrumentMap)
	instrumentGroup.GET("/options/exchange_names/:expiry", instrumentHandler.GetOptionsExchangeNames)
	instrumentGroup.GET("/options/:exchange/:name/:expiry", instrumentHandler.GetOptionChainInstruments)
	instrumentGroup.GET("/options/:exchange/:name/:expiry/full", instrumentHandler.GetOptionChainInstruments)

	// Instrument Indices routes (protected)
	indicesHandler := handlers.NewIndicesHandler(db)
	indicesGroup := api.Group("/indices")
	indicesGroup.Use(middleware.AuthMiddleware(db))
	indicesGroup.GET("/names/:exchange", indicesHandler.GetIndexNames)
	indicesGroup.GET("/instruments/:index", indicesHandler.GetIndexInstruments)
	indicesGroup.GET("/instruments/:index/full", indicesHandler.GetIndexInstruments)

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
