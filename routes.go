// Package main is the entry point for the Moneybots API
package main

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	handlerInstrument "github.com/nsvirk/moneybotsapi/api/instrument"
	handlerQuote "github.com/nsvirk/moneybotsapi/api/quote"
	handlerSession "github.com/nsvirk/moneybotsapi/api/session"
	handlerStream "github.com/nsvirk/moneybotsapi/api/stream"
	handlerTicker "github.com/nsvirk/moneybotsapi/api/ticker"
	"github.com/nsvirk/moneybotsapi/config"
	serviceSession "github.com/nsvirk/moneybotsapi/services/session"
	serviceTicker "github.com/nsvirk/moneybotsapi/services/ticker"
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
	sessionService := serviceSession.NewService(db)
	sessionHandler := handlerSession.NewHandler(sessionService)
	sessionGroup := api.Group("/session")
	sessionGroup.POST("/login", sessionHandler.GenerateSession)
	sessionGroup.POST("/totp", sessionHandler.GenerateTOTP)
	sessionGroup.POST("/valid", sessionHandler.CheckSessionValid)

	// Create a group for protected routes
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(sessionService))

	// Instrument routes (protected)
	instrumentHandler := handlerInstrument.NewHandler(db)
	instrumentGroup := protected.Group("/instrument")
	instrumentGroup.GET("/query", instrumentHandler.QueryInstruments)
	instrumentGroup.GET("/index/names", instrumentHandler.GetIndexNames)
	instrumentGroup.GET("/index", instrumentHandler.GetIndexInstruments)
	instrumentGroup.GET("/tokens", instrumentHandler.GetInstrumentToTokenMap)
	instrumentGroup.GET("/symbols", instrumentHandler.GetTokensToInstrumentMap)
	instrumentGroup.GET("/optionchain/names", instrumentHandler.GetOptionChainNames)
	instrumentGroup.GET("/optionchain", instrumentHandler.GetOptionChainInstruments)

	// Ticker routes (protected)
	tickerService := serviceTicker.NewService(db, redisClient)
	tickerHandler := handlerTicker.NewHandler(tickerService)
	tickerGroup := protected.Group("/ticker")
	tickerGroup.GET("/instruments", tickerHandler.GetTickerInstruments)
	tickerGroup.POST("/instruments", tickerHandler.AddTickerInstruments)
	tickerGroup.DELETE("/instruments", tickerHandler.DeleteTickerInstruments)
	tickerGroup.GET("/start", tickerHandler.TickerStart)
	tickerGroup.GET("/stop", tickerHandler.TickerStop)
	tickerGroup.GET("/restart", tickerHandler.TickerRestart)
	tickerGroup.GET("/status", tickerHandler.TickerStatus)

	// Quote routes (protected)
	quoteService := handlerQuote.NewService(db)
	quoteHandler := handlerQuote.NewHandler(quoteService)
	quoteGroup := protected.Group("/quote")
	quoteGroup.GET("", quoteHandler.GetQuote)
	quoteGroup.GET("/ohlc", quoteHandler.GetOHLC)
	quoteGroup.GET("/ltp", quoteHandler.GetLTP)

	// Stream routes (protected)
	streamHandler := handlerStream.NewHandler(db)
	streamGroup := protected.Group("/stream")
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
