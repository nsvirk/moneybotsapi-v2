// Package handlers contains the handlers for the API
package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"gorm.io/gorm"
)

type InstrumentHandler struct {
	DB                *gorm.DB
	InstrumentService *service.InstrumentService
	IndexService      *service.IndexService
}

func NewInstrumentHandler(db *gorm.DB) *InstrumentHandler {
	return &InstrumentHandler{
		DB:                db,
		InstrumentService: service.NewInstrumentService(db),
		IndexService:      service.NewIndexService(db),
	}
}

// UpdateInstrumentsResponseData is the response data for the UpdateInstruments endpoint
type UpdateInstrumentsResponseData struct {
	Timestamp string `json:"timestamp"`
	Records   int    `json:"records"`
}

// UpdateIndicesResponseData is the response data for the UpdateIndices endpoint
type UpdateIndicesResponseData struct {
	Timestamp string `json:"timestamp"`
	Records   int    `json:"records"`
}

// UpdateInstruments updates the instruments in the database
func (h *InstrumentHandler) UpdateInstruments(c echo.Context) error {
	totalInserted, err := h.InstrumentService.UpdateInstruments()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	responseData := UpdateInstrumentsResponseData{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Records:   totalInserted,
	}

	return response.SuccessResponse(c, responseData)
}

// QueryInstrumentsByExpiry returns a list of instruments for a given expiry and exchange
func (h *InstrumentHandler) QueryInstrumentsByExpiry(c echo.Context) error {
	expiry := c.FormValue("expiry")
	exchange := c.FormValue("exchange")

	if len(expiry) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `expiry` provided")
	}

	// check if expiry is valid date
	_, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `expiry` format")
	}

	instruments, err := h.InstrumentService.QueryInstrumentsByExpiry(expiry, exchange)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	if len(instruments) > 0 {
		// create a map of exchange to names
		responseData := make(map[string][]string, len(instruments))
		for _, instrument := range instruments {
			responseData[instrument.Exchange] = append(responseData[instrument.Exchange], instrument.Name)
		}
		return response.SuccessResponse(c, responseData)
	}

	return response.SuccessResponse(c, instruments)
}

// QueryInstruments returns a list of instruments for a given exchange, tradingsymbol, expiry, strike and segment
func (h *InstrumentHandler) QueryInstruments(c echo.Context) error {
	// get the exchange, tradingsymbol, expiry, strike and segment from the request
	exchange := c.FormValue("exchange")
	tradingsymbol := c.FormValue("tradingsymbol")
	name := c.FormValue("name")
	expiry := c.FormValue("expiry")
	strike := c.FormValue("strike")
	segment := c.FormValue("segment")
	instrumentType := c.FormValue("instrument_type")
	details := c.FormValue("details")

	// check if expiry is input and is a valid date
	if len(expiry) > 0 {
		_, err := time.Parse("2006-01-02", expiry)
		if err != nil {
			return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `expiry` value, must be a valid date")
		}
	}
	// check if strike is just digits if not blank
	if len(strike) > 0 && !regexp.MustCompile(`^\d+$`).MatchString(strike) {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `strike` value, must be digits")
	}

	// Check if instrument_type is one of FUT, CE, PE, EQ or include % anywhere in the string
	if len(instrumentType) > 0 && !regexp.MustCompile(`^(FUT|CE|PE|EQ)$|%`).MatchString(instrumentType) {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `instrument_type` value, must be `FUT`, `CE`, `PE` or `EQ` or include `%`")
	}

	// check if details is one of i, t, it
	if len(details) > 0 && !regexp.MustCompile(`^(i|t|it)$`).MatchString(details) {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `details` value, must be `i`, `t` or `it`")
	}

	queryInstrumentsParams := models.QueryInstrumentsParams{
		Exchange:       exchange,
		Tradingsymbol:  tradingsymbol,
		Name:           name,
		Expiry:         expiry,
		Strike:         strike,
		Segment:        segment,
		InstrumentType: instrumentType,
	}

	instruments, err := h.InstrumentService.QueryInstruments(queryInstrumentsParams, details)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instruments)
}

// UpdateIndices updates the indices in the database
func (h *InstrumentHandler) UpdateIndices(c echo.Context) error {
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

// GetIndexInstruments returns a list of instruments for a given list of index names
func (h *InstrumentHandler) GetIndexInstruments(c echo.Context) error {
	index := c.FormValue("index")

	if index == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `index` provided")
	}

	instruments, err := h.IndexService.GetNSEIndexInstruments(index)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", fmt.Sprintf("Error fetching instruments for index %s: %v", index, err))
	}

	return response.SuccessResponse(c, instruments)
}

// GetIndexNames returns a list of index names
func (h *InstrumentHandler) GetIndexNames(c echo.Context) error {
	indices, err := h.IndexService.GetNSEIndexNames()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}
	return response.SuccessResponse(c, indices)
}

// GetInstrumentSymbols returns a list of instrument symbols for a given list of instrument tokens
func (h *InstrumentHandler) GetTokensToInstrumentMap(c echo.Context) error {
	tokenParams := c.QueryParams()["t"]

	if len(tokenParams) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Input `t` is required")
	}

	var tokens []uint32
	for _, tokenStr := range tokenParams {
		token, err := strconv.ParseUint(tokenStr, 10, 32)
		if err != nil {
			return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `instrument_token` format")
		}
		tokens = append(tokens, uint32(token))
	}

	instrumentMap, err := h.InstrumentService.GetTokensToInstrumentMap(tokens)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instrumentMap)
}

// GetInstrumentTokens returns a list of instrument tokens for a given list of instruments
func (h *InstrumentHandler) GetInstrumentToTokenMap(c echo.Context) error {
	instruments := c.QueryParams()["i"]

	if len(instruments) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No instruments provided")
	}

	instrumentMap, err := h.InstrumentService.GetInstrumentToTokenMap(instruments)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instrumentMap)
}

// GetOptionChainNames returns a list of exchange:name for a given expiry
func (h *InstrumentHandler) GetOptionChainNames(c echo.Context) error {
	expiry := c.FormValue("expiry")
	if len(expiry) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `expiry` provided")
	}

	// check if expiry is valid date
	_, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `expiry` format")
	}

	instrumentNames, err := h.InstrumentService.GetOptionChainNames(expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	var responseData []string
	if len(instrumentNames) == 0 {
		responseData = []string{expiry}
	} else {
		responseData = instrumentNames
	}
	return response.SuccessResponse(c, responseData)
}

// GetOptionChainInstruments returns a list of instruments for a given exchange, name and expiry
func (h *InstrumentHandler) GetOptionChainInstruments(c echo.Context) error {
	exchange := c.FormValue("exchange")
	name := c.FormValue("name")
	expiry := c.FormValue("expiry")
	details := c.FormValue("details")

	// check if exchange is provided
	if len(exchange) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Input `exchange` is required")
	}

	// check if name is provided
	if len(name) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Input `name` is required")
	}

	// check if expiry is provided
	if len(expiry) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `expiry` provided")
	}

	// check if expiry is valid date
	_, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `expiry` format, must be a valid date")
	}

	// check if details is one of i, t, it
	if len(details) > 0 && !regexp.MustCompile(`^(i|t|it)$`).MatchString(details) {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `details` value, must be `i`, `t` or `it`")
	}

	instruments, err := h.InstrumentService.GetOptionChainInstruments(exchange, name, expiry, details)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instruments)

}
