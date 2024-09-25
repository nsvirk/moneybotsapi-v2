// Package handlers contains the handlers for the API
package handlers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"gorm.io/gorm"
)

// StreamHandler is the handler for the stream API
type StreamHandler struct {
	service *service.StreamService
}

// NewStreamHandler creates a new handler for the stream API
func NewStreamHandler(db *gorm.DB) *StreamHandler {
	return &StreamHandler{
		service: service.NewStreamService(db),
	}
}

type StreamRequestBody struct {
	Instruments []string `json:"instruments"`
}

// StreamTickerData streams the ticker data for the given instruments
func (h *StreamHandler) StreamTickerData(c echo.Context) error {
	userId, enctoken, err := extractAuthInfo(c)
	if err != nil {
		return err
	}

	var req StreamRequestBody
	if err := c.Bind(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid request body")
	}

	ctx := c.Request().Context()
	errChan := make(chan error, 1)

	go h.service.RunTickerStream(ctx, c, userId, enctoken, req.Instruments, errChan)

	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerError", fmt.Sprintf("Ticker error: %v", err))
	}
}
