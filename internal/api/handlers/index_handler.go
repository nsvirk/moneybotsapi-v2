// Package handlers contains the handlers for the API
package handlers

import (
	"fmt"
	"net/http"
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
	totalInserted, err := h.IndexService.UpdateNSEIndices()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	responseData := UpdateIndicesResponseData{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Records:   int(totalInserted),
	}

	return response.SuccessResponse(c, responseData)
}

// GetIndexNames returns a list of index names
func (h *IndicesHandler) GetIndexNames(c echo.Context) error {
	indices, err := h.IndexService.GetNSEIndexNames()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}
	return response.SuccessResponse(c, indices)
}

// GetIndexInstruments returns a list of instruments for a given list of index names
func (h *IndicesHandler) GetIndexInstruments(c echo.Context) error {
	index := c.FormValue("index")
	details := c.FormValue("details")

	if index == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `index` provided")
	}

	instruments, err := h.IndexService.GetNSEIndexInstruments(index, details)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", fmt.Sprintf("Error fetching instruments for index %s: %v", index, err))
	}

	return response.SuccessResponse(c, instruments)
}
