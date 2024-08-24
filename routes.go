// Package main is the entry point for the Moneybots API
package main

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/config"
	"github.com/nsvirk/moneybotsapi/instrument"
	"github.com/nsvirk/moneybotsapi/middleware"
	"github.com/nsvirk/moneybotsapi/quote"
	"github.com/nsvirk/moneybotsapi/session"
	"github.com/nsvirk/moneybotsapi/ticker"
	"github.com/nsvirk/moneybotsapi/utils"
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
	// instrumentGroup := api.Group("/instrument")
	instrumentGroup.POST("/update", instrumentHandler.UpdateInstruments)
	instrumentGroup.GET("/query", instrumentHandler.QueryInstruments)
	instrumentGroup.GET("/indices", instrumentHandler.GetIndicesInstruments)
	instrumentGroup.GET("/tokens", instrumentHandler.GetInstrumentTokens)
	instrumentGroup.GET("/symbols", instrumentHandler.GetInstrumentSymbols)

	// Ticker routes (protected)
	// Initialize ticker components
	tickerService := ticker.NewService(db, redisClient)
	tickerHandler := ticker.NewHandler(tickerService)
	tickerGroup := protected.Group("/ticker")
	// tickerGroup := api.Group("/ticker")
	tickerGroup.GET("/instruments", tickerHandler.GetInstruments)
	tickerGroup.POST("/instruments", tickerHandler.AddInstruments)
	tickerGroup.DELETE("/instruments", tickerHandler.DeleteInstruments)
	tickerGroup.GET("/start", tickerHandler.StartTicker)
	tickerGroup.GET("/stop", tickerHandler.StopTicker)
	tickerGroup.GET("/restart", tickerHandler.RestartTicker)

	// Quote routes (protected)
	quoteService := quote.NewService(db)
	quoteHandler := quote.NewHandler(quoteService)
	quoteGroup := api.Group("/quote")
	// quoteGroup := protected.Group("/quote")
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
	return utils.SuccessResponse(c, message)
}
