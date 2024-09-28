// Package handlers contains the handlers for the API
package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"gorm.io/gorm"
)

// UpdateIndicesResponseData is the response data for the UpdateIndices endpoint
type UpdateIndicesResponseData struct {
	Timestamp string `json:"timestamp"`
	Records   int    `json:"records"`
}

// IndicesHandler is the handler for the indices
type IndicesHandler struct {
	DB                *gorm.DB
	InstrumentService *service.InstrumentService
	IndexService      *service.IndexService
}

func NewIndicesHandler(db *gorm.DB) *IndicesHandler {
	return &IndicesHandler{
		DB:                db,
		InstrumentService: service.NewInstrumentService(db),
		IndexService:      service.NewIndexService(db),
	}
}

// UpdateIndices updates the indices in the database
func (h *IndicesHandler) UpdateIndices(c echo.Context) error {
	totalInserted, err := h.IndexService.UpdateIndices()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	responseData := UpdateIndicesResponseData{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Records:   int(totalInserted),
	}

	return response.SuccessResponse(c, responseData)
}

// GetIndexNames returns a list of index names for a given exchange
func (h *IndicesHandler) GetIndexNames(c echo.Context) error {
	exchange := c.Param("exchange")
	if exchange == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`exchange` is required")
	}

	indices, err := h.IndexService.GetIndexNames(exchange)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}
	return response.SuccessResponse(c, indices)
}

// GetIndexInstruments returns a list of instruments for a given list of index names
func (h *IndicesHandler) GetIndexInstruments(c echo.Context) error {
	index := c.Param("index")
	// details is optional and is only used for full
	urlPath := c.Request().URL.Path
	var details string
	if strings.Contains(urlPath, "/full") {
		details = "full"
	}

	if index == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`index` is required")
	}

	instruments, err := h.IndexService.GetIndexInstruments(index, details)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", fmt.Sprintf("Error fetching instruments for index %s: %v", index, err))
	}

	// make result as per details value
	result := make([]interface{}, len(instruments))
	if details == "full" {
		for i, instrument := range instruments {
			result[i] = instrument
		}
	} else {
		for i, instrument := range instruments {
			result[i] = fmt.Sprintf("%s:%s:%d", instrument.Exchange, instrument.Tradingsymbol, instrument.InstrumentToken)
		}
	}

	return response.SuccessResponse(c, result)
}
