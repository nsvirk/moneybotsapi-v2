// Package handlers contains the handlers for the API
package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
)

// QuoteHandler is the handler for the quote API
type QuoteHandler struct {
	service *service.QuoteService
}

// NewQuoteHandler creates a new quote handler
func NewQuoteHandler(service *service.QuoteService) *QuoteHandler {
	return &QuoteHandler{service: service}
}

// GetQuote gets the quote for the given instruments
func (h *QuoteHandler) GetQuote(c echo.Context) error {
	return h.handleRequest(c, mapTickToQuoteData)
}

// GetOHLC gets the OHLC data for the given instruments
func (h *QuoteHandler) GetOHLC(c echo.Context) error {
	return h.handleRequest(c, mapTickToOHLCData)
}

// GetLTP gets the LTP data for the given instruments
func (h *QuoteHandler) GetLTP(c echo.Context) error {
	return h.handleRequest(c, mapTickToLTPData)
}

// handleRequest is the common function to handle the request for the quote API
func (h *QuoteHandler) handleRequest(c echo.Context, mapper func(*models.TickerData) interface{}) error {
	instruments := c.QueryParams()["i"]
	if len(instruments) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No instruments specified")
	}

	tickDataMap, err := h.service.GetTickData(instruments)
	if err != nil {
		log.Printf("Error fetching tick data: %v", err)
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", fmt.Sprintf("Error fetching tick data: %v", err))
	}

	quoteResponse := models.QuoteResponse{
		Status: "success",
		Data:   make(map[string]interface{}),
	}

	for _, instrument := range instruments {
		if tickData, ok := tickDataMap[instrument]; ok {
			quoteResponse.Data[instrument] = mapper(tickData)
		}
	}

	if len(quoteResponse.Data) == 0 {
		return response.ErrorResponse(c, http.StatusNotFound, "DataNotFound", fmt.Sprintf("No data found for instruments: %v", instruments))
	}

	return c.JSON(http.StatusOK, quoteResponse)
}
