// File: github.com/nsvirk/moneybotsapi/instrument/handler.go

package instrument

import (
	"fmt"
	"net/http"
	"regexp"
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

func (h *Handler) GetIndexInstruments(c echo.Context) error {
	indices := c.QueryParams()["i"]

	if len(indices) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "No indices provided")
	}

	responseData := make(map[string][]string)

	for _, indexName := range indices {
		instruments, err := h.IndexService.GetIndexInstruments(indexName)
		if err != nil {
			return response.ErrorResponse(c, http.StatusInternalServerError, "fetch_error", fmt.Sprintf("Error fetching instruments for index %s: %v", indexName, err))
		}

		responseData[indexName] = instruments
	}

	return response.SuccessResponse(c, responseData)
}

func (h *Handler) GetIndexNames(c echo.Context) error {
	indices := h.IndexService.GetIndexNames()
	return response.SuccessResponse(c, indices)
}

func (h *Handler) QueryInstruments(c echo.Context) error {
	exchange := c.QueryParam("exchange")
	tradingsymbol := c.QueryParam("tradingsymbol")
	expiry := c.QueryParam("expiry")
	strike := c.QueryParam("strike")
	segment := c.QueryParam("segment")
	details := c.QueryParam("details")

	// check if exchange is only alphabets
	if !regexp.MustCompile(`^[A-Za-z]+$`).MatchString(exchange) {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid `exchange` format")
	}

	// check if tradingsymbol is only alphanumeric plus % and _
	if !regexp.MustCompile(`^[A-Za-z0-9%_]+$`).MatchString(tradingsymbol) {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid `tradingsymbol` format")
	}

	// check if expiry is valid date if not blank
	if expiry != "" {
		_, err := time.Parse("2006-01-02", expiry)
		if err != nil {
			return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid `expiry` format")
		}
	}

	// check if strike is just digits if not blank
	if strike != "" {
		if !regexp.MustCompile(`^\d+$`).MatchString(strike) {
			return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid `strike` format")
		}
	}

	detailsBool, err := strconv.ParseBool(details)
	if details != "" {
		if err != nil {
			return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid details value")
		}
	}

	instruments, err := h.InstrumentService.QueryInstruments(exchange, tradingsymbol, expiry, strike, segment)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "query_error", err.Error())
	}

	if detailsBool {
		return response.SuccessResponse(c, instruments)
	} else {
		instrumentsList := make([]string, len(instruments))
		for i, inst := range instruments {
			instrumentsList[i] = fmt.Sprintf("%s:%s", inst.Exchange, inst.Tradingsymbol)
		}
		return response.SuccessResponse(c, instrumentsList)
	}
}

func (h *Handler) GetInstrumentSymbols(c echo.Context) error {
	tokenParams := c.QueryParams()["t"]

	if len(tokenParams) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Input `t` is required")
	}

	var tokens []uint32
	for _, tokenStr := range tokenParams {
		token, err := strconv.ParseUint(tokenStr, 10, 32)
		if err != nil {
			return response.ErrorResponse(c, http.StatusBadRequest, "invalid_token", "Invalid instrument token format")
		}
		tokens = append(tokens, uint32(token))
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

	instrumentMap, err := h.InstrumentService.GetInstrumentToTokenMap(instruments)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "query_error", err.Error())
	}

	return response.SuccessResponse(c, instrumentMap)
}

func (h *Handler) GetOptionChainNames(c echo.Context) error {
	expiry := c.QueryParam("expiry")
	if len(expiry) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "No `expiry` provided")
	}

	// check if expiry is valid date
	_, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid `expiry` format")
	}

	names, err := h.InstrumentService.GetOptionChainNames(expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "query_error", err.Error())
	}

	return response.SuccessResponse(c, names)
}

func (h *Handler) GetOptionChainInstruments(c echo.Context) error {
	name := c.QueryParam("name")
	expiry := c.QueryParam("expiry")
	details := c.QueryParam("details")

	if len(name) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Input `name` is required")
	}

	if len(expiry) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "No `expiry` provided")
	}
	// check if expiry is valid date
	_, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid `expiry` format")
	}

	detailsBool, err := strconv.ParseBool(details)
	if details != "" {
		if err != nil {
			return response.ErrorResponse(c, http.StatusBadRequest, "invalid_request", "Invalid `details` value")
		}
	}

	instrumentsMap, err := h.InstrumentService.GetOptionChainInstruments(name, expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "query_error", err.Error())
	}

	if detailsBool {
		return response.SuccessResponse(c, instrumentsMap)

	} else {
		instrumentsMapList := make(map[string][]string, 0)
		for name, instruments := range instrumentsMap {
			for _, instrument := range instruments {
				instrumentsMapList[name] = append(instrumentsMapList[name], fmt.Sprintf("%s:%s", instrument.Exchange, instrument.Tradingsymbol))
			}
		}
		return response.SuccessResponse(c, instrumentsMapList)
	}

}
