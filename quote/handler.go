package quote

import (
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/ticker"
	"github.com/nsvirk/moneybotsapi/utils"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetQuote(c echo.Context) error {
	return h.handleRequest(c, mapTickToQuoteData)
}

func (h *Handler) GetOHLC(c echo.Context) error {
	return h.handleRequest(c, mapTickToOHLCData)
}

func (h *Handler) GetLTP(c echo.Context) error {
	return h.handleRequest(c, mapTickToLTPData)
}

func (h *Handler) handleRequest(c echo.Context, mapper func(*ticker.TickerData) interface{}) error {
	instruments := c.QueryParams()["i"]
	if len(instruments) == 0 {
		return utils.ErrorResponse(c, http.StatusBadRequest, "InputException", "No instruments specified")
	}

	tickDataMap, err := h.service.GetTickData(instruments)
	if err != nil {
		log.Printf("Error fetching tick data: %v", err)
		return utils.ErrorResponse(c, http.StatusInternalServerError, "ServerException", fmt.Sprintf("Error fetching tick data: %v", err))
	}

	response := QuoteResponse{
		Status: "success",
		Data:   make(map[string]interface{}),
	}

	for _, instrument := range instruments {
		if tickData, ok := tickDataMap[instrument]; ok {
			response.Data[instrument] = mapper(tickData)
		}
	}

	if len(response.Data) == 0 {
		return utils.ErrorResponse(c, http.StatusNotFound, "DataNotFound", fmt.Sprintf("No data found for instruments: %v", instruments))
	}

	return c.JSON(http.StatusOK, response)
}
