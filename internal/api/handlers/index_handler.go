// Package handlers contains the handlers for the API
package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"gorm.io/gorm"
)

// UpdateIndexResponseData is the response data for the UpdateIndex endpoint
type UpdateIndexResponseData struct {
	Timestamp string `json:"timestamp"`
	Records   int    `json:"records"`
}

// IndexHandler is the handler for the indices
type IndexHandler struct {
	DB                *gorm.DB
	InstrumentService *service.InstrumentService
	IndexService      *service.IndexService
}

func NewIndexHandler(db *gorm.DB) *IndexHandler {
	return &IndexHandler{
		DB:                db,
		InstrumentService: service.NewInstrumentService(db),
		IndexService:      service.NewIndexService(db),
	}
}

// UpdateIndices updates the indices in the database
func (h *IndexHandler) UpdateIndices(c echo.Context) error {
	totalInserted, err := h.IndexService.UpdateIndices()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}
	responseData := UpdateIndexResponseData{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Records:   int(totalInserted),
	}
	return response.SuccessResponse(c, responseData)
}

// GetAllIndices returns a list of all indices
func (h *IndexHandler) GetAllIndices(c echo.Context) error {
	indices, err := h.IndexService.GetAllIndices()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}
	result := make(map[string][]models.IndexModel, len(indices))
	for _, index := range indices {
		result[index.Exchange] = append(result[index.Exchange], index)
	}
	return response.SuccessResponse(c, result)
}

// GetIndicesByExchange returns a list of indices for a given exchange
func (h *IndexHandler) GetIndicesByExchange(c echo.Context) error {
	exchange := c.Param("exchange")
	if exchange == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`exchange` is required")
	}
	indices, err := h.IndexService.GetIndicesByExchange(exchange)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}
	return response.SuccessResponse(c, indices)
}

// GetIndexInstruments returns a list of instruments for a given list of index names
func (h *IndexHandler) GetIndexInstruments(c echo.Context) error {
	exchange := c.Param("exchange")
	index := c.Param("index")
	if exchange == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`exchange` is required")
	}
	if index == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`index` is required")
	}
	instruments, err := h.IndexService.GetIndexInstruments(exchange, index)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", fmt.Sprintf("Error fetching instruments for index %s: %v", index, err))
	}
	return response.SuccessResponse(c, instruments)
}
