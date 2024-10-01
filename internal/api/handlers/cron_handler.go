// Package handlers contains the handlers for the API
package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/config"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type CronHandler struct {
	DB          *gorm.DB
	CronService *service.CronService
}

func NewCronHandler(e *echo.Echo, cfg *config.Config, db *gorm.DB, redisClient *redis.Client) *CronHandler {
	return &CronHandler{
		DB:          db,
		CronService: service.NewCronService(e, cfg, db, redisClient),
	}
}

// UpdateInstruments updates the instruments
func (h *CronHandler) UpdateInstruments(c echo.Context) error {
	h.CronService.ApiInstrumentsUpdateJob()
	return response.SuccessResponse(c, "Instruments updated")
}

func (h *CronHandler) UpdateIndices(c echo.Context) error {
	h.CronService.ApiIndicesUpdateJob()
	return response.SuccessResponse(c, "Indices updated")
}

// TickerInstrumentsUpdateJob updates the ticker instruments
func (h *CronHandler) TickerInstrumentsUpdateJob(c echo.Context) error {
	h.CronService.TickerInstrumentsUpdateJob()
	return response.SuccessResponse(c, "Ticker instruments updated")
}

// TickerStartJob starts the ticker
func (h *CronHandler) TickerStartJob(c echo.Context) error {
	h.CronService.TickerStartJob()
	return response.SuccessResponse(c, "Ticker started")
}

// TickerStopJob stops the ticker
func (h *CronHandler) TickerStopJob(c echo.Context) error {
	h.CronService.TickerStopJob()
	return response.SuccessResponse(c, "Ticker stopped")
}
