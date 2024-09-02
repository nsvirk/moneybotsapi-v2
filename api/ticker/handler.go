package ticker

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/shared/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) TickerStart(c echo.Context) error {
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

func (h *Handler) TickerStop(c echo.Context) error {
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

func (h *Handler) TickerRestart(c echo.Context) error {
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

// Status returns the current status of the ticker
func (h *Handler) TickerStatus(c echo.Context) error {
	status := h.service.Status()
	return response.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    status,
	})
}

func (h *Handler) GetTickerInstruments(c echo.Context) error {
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

func (h *Handler) AddTickerInstruments(c echo.Context) error {
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

func (h *Handler) DeleteTickerInstruments(c echo.Context) error {
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

func extractAuthInfo(c echo.Context) (string, string, error) {
	auth := c.Request().Header.Get("Authorization")
	userID, enctoken, found := strings.Cut(auth, ":")
	if !found {
		return "", "", response.ErrorResponse(c, http.StatusUnauthorized, "InputException", "Invalid authorization header")
	}
	return userID, enctoken, nil
}
