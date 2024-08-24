package ticker

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/utils"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) StartTicker(c echo.Context) error {
	userID, enctoken, err := extractAuthInfo(c)
	if err != nil {
		return err
	}

	if err := h.service.Start(userID, enctoken); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "TickerException", err.Error())
	}

	instruments, err := h.service.GetTickerInstruments()
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"records":   len(instruments),
		"message":   "started",
	})
}

func (h *Handler) StopTicker(c echo.Context) error {
	if err := h.service.Stop(); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "InputException", err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "stopped",
	})
}

func (h *Handler) RestartTicker(c echo.Context) error {
	userID, enctoken, err := extractAuthInfo(c)
	if err != nil {
		return err
	}

	if err := h.service.Restart(userID, enctoken); err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "TickerException", err.Error())
	}

	instruments, err := h.service.GetTickerInstruments()
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"records":   len(instruments),
		"message":   "restarted",
	})
}

func (h *Handler) GetInstruments(c echo.Context) error {
	instruments, err := h.service.GetTickerInstruments()
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", "Failed to fetch instruments")
	}

	respInstruments := make([]string, len(instruments))
	for i, instrument := range instruments {
		respInstruments[i] = instrument.Instrument
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"records":     len(instruments),
		"instruments": respInstruments,
	})
}

func (h *Handler) AddInstruments(c echo.Context) error {
	var req struct {
		Instruments []string `json:"instruments"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid JSON body")
	}

	response, err := h.service.AddTickerInstruments(req.Instruments)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	totalCount, _ := h.service.GetTickerInstrumentCount()

	return utils.SuccessResponse(c, map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"records":     totalCount,
		"instruments": response,
	})
}

func (h *Handler) DeleteInstruments(c echo.Context) error {
	var req struct {
		Instruments []string `json:"instruments"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid JSON body")
	}

	deletedCount, err := h.service.DeleteTickerInstruments(req.Instruments)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, "DatabaseException", err.Error())
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"records":   deletedCount,
	})
}

func extractAuthInfo(c echo.Context) (string, string, error) {
	auth := c.Request().Header.Get("Authorization")
	userID, enctoken, found := strings.Cut(auth, ":")
	if !found {
		return "", "", utils.ErrorResponse(c, http.StatusUnauthorized, "InputException", "Invalid authorization header")
	}
	return userID, enctoken, nil
}
