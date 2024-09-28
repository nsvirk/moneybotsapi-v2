// Package handlers contains the handlers for the API
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/api/middleware"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
)

// TickerHandler is the handler for the ticker API
type TickerHandler struct {
	service *service.TickerService
}

// NewTickerHandler creates a new handler for the ticker API
func NewTickerHandler(service *service.TickerService) *TickerHandler {
	return &TickerHandler{service: service}
}

// TickerStart starts the ticker for the given user
func (h *TickerHandler) TickerStart(c echo.Context) error {
	userId, enctoken, err := middleware.GetUserIdEnctokenFromEchoContext(c)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", err.Error())
	}

	if err := h.service.Start(userId, enctoken); err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "TickerException", err.Error())
	}

	instruments, err := h.service.GetTickerInstruments(userId)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"records":   len(instruments),
		"message":   "started",
	})
}

// TickerStop stops the ticker for the given user
func (h *TickerHandler) TickerStop(c echo.Context) error {
	userId, _, err := middleware.GetUserIdEnctokenFromEchoContext(c)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", err.Error())
	}

	if err := h.service.Stop(userId); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", err.Error())
	}

	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "stopped",
	})
}

// TickerRestart restarts the ticker for the given user
func (h *TickerHandler) TickerRestart(c echo.Context) error {
	userId, enctoken, err := middleware.GetUserIdEnctokenFromEchoContext(c)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", err.Error())
	}

	if err := h.service.Restart(userId, enctoken); err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "TickerException", err.Error())
	}

	instruments, err := h.service.GetTickerInstruments(userId)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"records":   len(instruments),
		"message":   "restarted",
	})
}

// TickerStatus returns the current status of the ticker
func (h *TickerHandler) TickerStatus(c echo.Context) error {
	status := h.service.Status()
	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    status,
	})
}

// GetTickerInstruments returns the instruments for the given user
func (h *TickerHandler) GetTickerInstruments(c echo.Context) error {
	userId, _, err := middleware.GetUserIdEnctokenFromEchoContext(c)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", err.Error())
	}

	tickerInstruments, err := h.service.GetTickerInstruments(userId)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", "Failed to fetch instruments")
	}

	respTickerInstruments := make([]string, len(tickerInstruments))
	for i, instrument := range tickerInstruments {
		respTickerInstruments[i] = instrument.Instrument
	}

	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"records":     len(tickerInstruments),
		"instruments": respTickerInstruments,
	})
}

// AddTickerInstruments adds the given instruments to the ticker for the given user
func (h *TickerHandler) AddTickerInstruments(c echo.Context) error {
	userId, _, err := middleware.GetUserIdEnctokenFromEchoContext(c)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", err.Error())
	}
	var req struct {
		Instruments []string `json:"instruments"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid JSON body")
	}

	instruments, err := h.service.AddTickerInstruments(userId, req.Instruments)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	totalCount, _ := h.service.GetTickerInstrumentCount(userId)

	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"records":     totalCount,
		"instruments": instruments,
	})
}

// DeleteTickerInstruments deletes the given instruments from the ticker for the given user
func (h *TickerHandler) DeleteTickerInstruments(c echo.Context) error {
	userId, _, err := middleware.GetUserIdEnctokenFromEchoContext(c)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", err.Error())
	}
	var req struct {
		Instruments []string `json:"instruments"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid JSON body")
	}

	// Add validation for empty instruments array
	if len(req.Instruments) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Instruments array cannot be empty")
	}

	deletedCount, err := h.service.DeleteTickerInstruments(userId, req.Instruments)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	return response.SuccessResponse(c, map[string]interface{}{
		"deleted":   deletedCount,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
