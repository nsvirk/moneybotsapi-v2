// Package handlers contains the handlers for the API
package handlers

import (
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

// UpdateInstruments updates the instruments in the database
func (h *InstrumentHandler) UpdateInstruments(c echo.Context) error {
	totalInserted, err := h.InstrumentService.UpdateInstruments()
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	responseData := UpdateInstrumentsResponseData{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Records:   int(totalInserted),
	}

	return response.SuccessResponse(c, responseData)
}

// GetInstrumentsByExchange queries the instruments table by exchange and returns a list of instruments
func (h *InstrumentHandler) GetInstrumentsByExchange(c echo.Context) error {
	exchange := c.Param("exchange")

	if len(exchange) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `exchange` provided")
	}

	instruments, err := h.InstrumentService.GetInstrumentsByExchange(exchange)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instruments)
}

// GetInstrumentsByTradingsymbol queries the instruments table by tradingsymbol and returns a list of instruments
func (h *InstrumentHandler) GetInstrumentsByTradingsymbol(c echo.Context) error {

	tradingsymbol := c.Param("tradingsymbol")

	if len(tradingsymbol) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `exchange` provided")
	}

	instruments, err := h.InstrumentService.GetInstrumentsByTradingsymbol(tradingsymbol)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instruments)
}

// GetInstrumentsByInstrumentToken queries the instruments table by instrument token and returns a list of instruments
func (h *InstrumentHandler) GetInstrumentsByInstrumentToken(c echo.Context) error {
	instrumentToken := c.Param("instrument_token")

	if instrumentToken == "" || instrumentToken == ":instrument_token" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `instrument_token` provided")
	}

	// check if its a digit
	if !regexp.MustCompile(`^\d+$`).MatchString(instrumentToken) {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `instrument_token` format")
	}

	instruments, err := h.InstrumentService.GetInstrumentsByInstrumentToken(instrumentToken)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instruments)
}

// GetInstrumentsByExpiry queries the instruments table by expiry and returns a list of instruments
func (h *InstrumentHandler) GetInstrumentsByExpiry(c echo.Context) error {
	expiry := c.Param("expiry")

	if len(expiry) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `expiry` provided")
	}

	instruments, err := h.InstrumentService.GetInstrumentsByExpiry(expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instruments)
}

// GetExchangeNamesByExpiry queries the instruments table by expiry and returns a list of distinct exchange, names
func (h *InstrumentHandler) GetExchangeNamesByExpiry(c echo.Context) error {
	expiry := c.Param("expiry")

	if len(expiry) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `expiry` provided")
	}

	// check if expiry is valid date
	_, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `expiry` format")
	}

	instruments, err := h.InstrumentService.GetExchangeNamesByExpiry(expiry)
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
