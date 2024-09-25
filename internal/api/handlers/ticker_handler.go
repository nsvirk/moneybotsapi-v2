// Package handlers contains the handlers for the API
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
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
	userID, enctoken, err := extractAuthInfo(c)
	if err != nil {
		return err
	}

	if err := h.service.Start(userID, enctoken); err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "TickerException", err.Error())
	}

	instruments, err := h.service.GetTickerInstruments(userID)
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
	userID, _, err := extractAuthInfo(c)
	if err != nil {
		return err
	}

	if err := h.service.Stop(userID); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", err.Error())
	}

	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "stopped",
	})
}

// TickerRestart restarts the ticker for the given user
func (h *TickerHandler) TickerRestart(c echo.Context) error {
	userID, enctoken, err := extractAuthInfo(c)
	if err != nil {
		return err
	}

	if err := h.service.Restart(userID, enctoken); err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "TickerException", err.Error())
	}

	instruments, err := h.service.GetTickerInstruments(userID)
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
	userID, _, err := extractAuthInfo(c)
	if err != nil {
		return err
	}
	tickerInstruments, err := h.service.GetTickerInstruments(userID)
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
	userID, _, err := extractAuthInfo(c)
	if err != nil {
		return err
	}
	var req struct {
		Instruments []string `json:"instruments"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid JSON body")
	}

	instruments, err := h.service.AddTickerInstruments(userID, req.Instruments)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	totalCount, _ := h.service.GetTickerInstrumentCount(userID)

	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"records":     totalCount,
		"instruments": instruments,
	})
}

// DeleteTickerInstruments deletes the given instruments from the ticker for the given user
func (h *TickerHandler) DeleteTickerInstruments(c echo.Context) error {
	userID, _, err := extractAuthInfo(c)
	if err != nil {
		return err
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

	deletedCount, err := h.service.DeleteTickerInstruments(userID, req.Instruments)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	return response.SuccessResponse(c, map[string]interface{}{
		"deleted":   deletedCount,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// extractAuthInfo extracts the userID and enctoken from the authorization header
func extractAuthInfo(c echo.Context) (string, string, error) {
	auth := c.Request().Header.Get("Authorization")
	userID, enctoken, found := strings.Cut(auth, ":")
	if !found {
		return "", "", response.ErrorResponse(c, http.StatusUnauthorized, "InputException", "Invalid authorization header")
	}
	return userID, enctoken, nil
}
