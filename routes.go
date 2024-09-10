// Package main is the entry point for the Moneybots API
package main

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/api/instrument"
	"github.com/nsvirk/moneybotsapi/api/quote"
	"github.com/nsvirk/moneybotsapi/api/session"
	"github.com/nsvirk/moneybotsapi/api/ticker"
	"github.com/nsvirk/moneybotsapi/config"
	"github.com/nsvirk/moneybotsapi/shared/middleware"
	"github.com/nsvirk/moneybotsapi/shared/response"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// setupRoutes configures the routes for the API
func setupRoutes(e *echo.Echo, db *gorm.DB, redisClient *redis.Client) {

	// Create a group for all API routes
	api := e.Group("/api")

	// Index route
	api.GET("/", indexRoute)

	// Session routes - Unprotected
	sessionService := session.NewService(db)
	sessionHandler := session.NewHandler(sessionService)
	sessionGroup := api.Group("/session")
	sessionGroup.POST("/login", sessionHandler.GenerateSession)
	sessionGroup.POST("/totp", sessionHandler.GenerateTOTP)
	sessionGroup.POST("/valid", sessionHandler.CheckSessionValid)

	// Create a group for protected routes
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(sessionService))

	// Instrument routes (protected)
	instrumentHandler := instrument.NewHandler(db)
	instrumentGroup := protected.Group("/instrument")
	// instrumentGroup := api.Group("/instrument") // for debugging
	instrumentGroup.POST("/update", instrumentHandler.UpdateInstruments)
	instrumentGroup.GET("/query", instrumentHandler.QueryInstruments)
	instrumentGroup.GET("/indices", instrumentHandler.GetIndicesInstruments)
	instrumentGroup.GET("/tokens", instrumentHandler.GetInstrumentTokens)
	instrumentGroup.GET("/symbols", instrumentHandler.GetInstrumentSymbols)
	instrumentGroup.GET("/optionchain/names", instrumentHandler.GetOptionChainNames)
	instrumentGroup.GET("/optionchain/instruments", instrumentHandler.GetOptionChainInstruments)

	// Ticker routes (protected)
	// Initialize ticker components
	tickerService := ticker.NewService(db, redisClient)
	tickerHandler := ticker.NewHandler(tickerService)
	tickerGroup := protected.Group("/ticker")
	// tickerGroup := api.Group("/ticker") // for debugging
	tickerGroup.GET("/instruments", tickerHandler.GetTickerInstruments)
	tickerGroup.POST("/instruments", tickerHandler.AddTickerInstruments)
	tickerGroup.DELETE("/instruments", tickerHandler.DeleteTickerInstruments)
	tickerGroup.GET("/start", tickerHandler.TickerStart)
	tickerGroup.GET("/stop", tickerHandler.TickerStop)
	tickerGroup.GET("/restart", tickerHandler.TickerRestart)
	tickerGroup.GET("/status", tickerHandler.TickerStatus)

	// Quote routes (protected)
	quoteService := quote.NewService(db)
	quoteHandler := quote.NewHandler(quoteService)
	// quoteGroup := api.Group("/quote") // for debugging
	quoteGroup := protected.Group("/quote")
	quoteGroup.GET("", quoteHandler.GetQuote)
	quoteGroup.GET("/ohlc", quoteHandler.GetOHLC)
	quoteGroup.GET("/ltp", quoteHandler.GetLTP)

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
