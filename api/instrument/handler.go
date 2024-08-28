// File: github.com/nsvirk/moneybotsapi/instrument/handler.go

package instrument

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/shared/response"
	"gorm.io/gorm"
)

type Handler struct {
	DB                *gorm.DB
	InstrumentService *InstrumentService
	IndexService      *IndexService
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		DB:                db,
		InstrumentService: NewInstrumentService(db),
		IndexService:      NewIndexService(),
	}
}

type UpdateInstrumentsResponseData struct {
	Timestamp string `json:"timestamp"`
	Records   int    `json:"records"`
}

func (h *Handler) UpdateInstruments(c echo.Context) error {
	totalInserted, err := h.InstrumentService.UpdateInstruments()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "update_error", err.Error())
	}

	responseData := UpdateInstrumentsResponseData{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Records:   totalInserted,
	}

	return response.SuccessResponse(c, responseData)
}

func (h *Handler) GetIndicesInstruments(c echo.Context) error {
	indices := c.QueryParams()["i"]

	if len(indices) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "No indices provided")
	}

	responseData := make(map[string][]string)

	for _, indexName := range indices {
		instruments, err := h.IndexService.FetchIndexInstrumentsList(indexName)
		if err != nil {
			return response.ErrorResponse(c, http.StatusInternalServerError, "fetch_error", fmt.Sprintf("Error fetching instruments for index %s: %v", indexName, err))
		}

		responseData[indexName] = instruments
	}

	return response.SuccessResponse(c, responseData)
}

func (h *Handler) GetAvailableIndices(c echo.Context) error {
	indices := h.IndexService.GetAvailableIndices()
	return response.SuccessResponse(c, indices)
}

func (h *Handler) QueryInstruments(c echo.Context) error {
	exchange := c.QueryParam("exchange")
	tradingsymbol := c.QueryParam("tradingsymbol")
	expiry := c.QueryParam("expiry")
	strike := c.QueryParam("strike")
	instrumentsOnly := c.QueryParam("instruments_only")

	instrumentsOnlyBool, err := strconv.ParseBool(instrumentsOnly)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid instruments_only value")
	}

	instruments, err := h.InstrumentService.QueryInstruments(exchange, tradingsymbol, expiry, strike)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "query_error", err.Error())
	}

	if instrumentsOnlyBool {
		instrumentsList := make([]string, len(instruments))
		for i, inst := range instruments {
			instrumentsList[i] = fmt.Sprintf("%s:%s", inst.Exchange, inst.Tradingsymbol)
		}
		return response.SuccessResponse(c, instrumentsList)
	} else {
		instrumentMap := make(map[string]interface{})
		for _, inst := range instruments {
			symbol := fmt.Sprintf("%s:%s", inst.Exchange, inst.Tradingsymbol)
			instrumentMap[symbol] = map[string]interface{}{
				"exchange":         inst.Exchange,
				"expiry":           inst.Expiry,
				"instrument_token": inst.InstrumentToken,
				"strike":           inst.Strike,
				"tradingsymbol":    inst.Tradingsymbol,
			}
		}
		return response.SuccessResponse(c, instrumentMap)
	}
}

func (h *Handler) GetInstrumentSymbols(c echo.Context) error {
	tokenParams := c.QueryParams()["t"]

	if len(tokenParams) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "No instrument tokens provided")
	}

	var tokens []uint
	for _, tokenStr := range tokenParams {
		token, err := strconv.ParseUint(tokenStr, 10, 32)
		if err != nil {
			return response.ErrorResponse(c, http.StatusBadRequest, "invalid_token", "Invalid instrument token format")
		}
		tokens = append(tokens, uint(token))
	}

	instrumentMap, err := h.InstrumentService.GetInstrumentSymbols(tokens)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "query_error", err.Error())
	}

	return response.SuccessResponse(c, instrumentMap)
}

func (h *Handler) GetInstrumentTokens(c echo.Context) error {
	instruments := c.QueryParams()["i"]

	if len(instruments) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "No instruments provided")
	}

	instrumentMap, err := h.InstrumentService.GetInstrumentTokens(instruments)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "query_error", err.Error())
	}

	return response.SuccessResponse(c, instrumentMap)
}
