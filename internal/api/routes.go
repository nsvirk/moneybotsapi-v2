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
func SetupRoutes(e *echo.Echo, cfg *config.Config, db *gorm.DB, redisClient *redis.Client) {

	// Create a group for all API routes
	api := e.Group("")

	// Index route
	api.GET("/", indexRoute)

	// Session routes (unprotected)
	sessionService := service.NewSessionService(db)
	sessionHandler := handlers.NewSessionHandler(sessionService)
	sessionGroup := api.Group("/session")
	sessionGroup.POST("/token", sessionHandler.GenerateSession)
	sessionGroup.DELETE("/token", sessionHandler.DeleteSession)
	sessionGroup.POST("/totp", sessionHandler.GenerateTOTP)
	sessionGroup.POST("/valid", sessionHandler.CheckEnctokenValid)

	// Instrument routes (protected)
	instrumentHandler := handlers.NewInstrumentHandler(db)
	instrumentGroup := api.Group("/instruments")
	instrumentGroup.Use(middleware.AuthMiddleware(db))
	// instrument routes
	instrumentGroup.GET("/info", instrumentHandler.GetInstrumentsInfo)
	instrumentGroup.GET("/query", instrumentHandler.GetInstrumentsQuery)
	// instrument fno routes
	instrumentGroup.GET("/fno/segment_expiries/:name", instrumentHandler.GetFNOSegmentWiseExpiry)
	instrumentGroup.GET("/fno/segment_names/:expiry", instrumentHandler.GetFNOSegmentWiseName)
	instrumentGroup.GET("/fno/optionchain", instrumentHandler.GetFNOOptionChain)

	// Indices routes (protected)
	indexHandler := handlers.NewIndexHandler(db)
	indexGroup := api.Group("/indices")
	indexGroup.Use(middleware.AuthMiddleware(db))
	indexGroup.GET("/all", indexHandler.GetAllIndices)
	indexGroup.GET("/:exchange/info", indexHandler.GetIndicesByExchange)
	indexGroup.GET("/:exchange/:index/instruments", indexHandler.GetIndexInstruments)

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

	// Cron routes (protected)
	cronHandler := handlers.NewCronHandler(e, cfg, db, redisClient)
	cronGroup := api.Group("/cron")
	cronGroup.Use(middleware.AuthMiddleware(db))
	cronGroup.PUT("/indices", cronHandler.UpdateIndices)
	cronGroup.PUT("/instruments", cronHandler.UpdateInstruments)
	cronGroup.PUT("/ticker_instruments", cronHandler.TickerInstrumentsUpdateJob)
	// cronGroup.GET("/ticker_start", cronHandler.TickerStartJob)
	// cronGroup.GET("/ticker_stop", cronHandler.TickerStopJob)
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
