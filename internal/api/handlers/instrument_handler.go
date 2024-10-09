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

// GetInstrumentsInfo returns instruments by symbols or tokens
func (h *InstrumentHandler) GetInstrumentsInfo(c echo.Context) error {
	symbols := c.QueryParams()["s"]
	tokensStr := c.QueryParams()["t"]
	// check if symbols or tokensStr is provided
	if len(symbols) == 0 && len(tokensStr) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`s` or `t` is required")
	}
	// check if both symbols and tokensStr is provided
	if len(symbols) > 0 && len(tokensStr) > 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Either `s` or `t` is required, not both")
	}
	// create a map to store the result
	result := make(map[string]interface{})
	// get instruments for symbols or tokens
	if len(symbols) > 0 {
		symbolInstruments, err := h.InstrumentService.GetInstrumentsInfoBySymbols(symbols)
		if err != nil {
			return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
		}
		for _, instrument := range symbolInstruments {
			result[fmt.Sprintf("%s:%s", instrument.Exchange, instrument.Tradingsymbol)] = instrument
		}
	} else if len(tokensStr) > 0 {
		// convert tokensStr to []uint32
		var tokens []uint32
		for _, tokenStr := range tokensStr {
			token, err := strconv.ParseUint(tokenStr, 10, 32)
			if err != nil {
				return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `instrument_token`, must be digits")
			}
			tokens = append(tokens, uint32(token))
		}
		tokenInstruments, err := h.InstrumentService.GetInstrumentsInfoByTokens(tokens)
		if err != nil {
			return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
		}
		for _, instrument := range tokenInstruments {
			result[fmt.Sprintf("%d", instrument.InstrumentToken)] = instrument
		}
	}
	return response.SuccessResponse(c, result)
}

// GetInstrumentsQuery returns a list of instruments for a given exchange, tradingsymbol, expiry, strike and segment
func (h *InstrumentHandler) GetInstrumentsQuery(c echo.Context) error {
	// get the exchange, tradingsymbol, instrument_token, name, expiry, strike and segment from the request
	exchange := c.QueryParam("exchange")
	tradingsymbol := c.QueryParam("tradingsymbol")
	instrumentToken := c.QueryParam("instrument_token")
	name := c.QueryParam("name")
	expiry := c.QueryParam("expiry")
	strike := c.QueryParam("strike")
	segment := c.QueryParam("segment")
	instrumentType := c.QueryParam("instrument_type")
	// check instrumentToken is all digits
	if len(instrumentToken) > 0 && !regexp.MustCompile(`^\d+$`).MatchString(instrumentToken) {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `instrument_token` value, must be digits")
	}
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
	// Create the query instruments params
	queryInstrumentsParams := models.QueryInstrumentsParams{
		Exchange:        exchange,
		Tradingsymbol:   tradingsymbol,
		InstrumentToken: instrumentToken,
		Name:            name,
		Expiry:          expiry,
		Strike:          strike,
		Segment:         segment,
		InstrumentType:  instrumentType,
	}
	// get the instruments
	instruments, err := h.InstrumentService.GetInstrumentsQuery(queryInstrumentsParams)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}
	return response.SuccessResponse(c, instruments)
}

// GetFNOSegmentWiseName returns a list of segment wise name for a given expiry
func (h *InstrumentHandler) GetFNOSegmentWiseName(c echo.Context) error {
	expiry := c.Param("expiry")
	if len(expiry) == 0 || expiry == ":expiry" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`expiry` is required")
	}

	// check if expiry is valid date
	_, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `expiry` format")
	}

	instruments, err := h.InstrumentService.GetFNOSegmentWiseName(expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	// create a map of segment to names
	if len(instruments) > 0 {
		responseData := make(map[string][]string, len(instruments))
		for _, instrument := range instruments {
			responseData[instrument.Segment] = append(responseData[instrument.Segment], instrument.Name)
		}
		return response.SuccessResponse(c, responseData)

	} else {
		return response.SuccessResponse(c, instruments)
	}
}

// GetFNOSegmentExpiry returns the expiry for a given exchange, name
func (h *InstrumentHandler) GetFNOSegmentWiseExpiry(c echo.Context) error {
	name := c.Param("name")
	var limit int = 20
	var offset int = 0

	if len(name) == 0 || name == ":name" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`name` is required")
	}

	instruments, err := h.InstrumentService.GetFNOSegmentWiseExpiry(name, limit, offset)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	// create a map of names to segments
	if len(instruments) > 0 {
		responseData := make(map[string][]string, len(instruments))
		for _, instrument := range instruments {
			responseData[instrument.Segment] = append(responseData[instrument.Segment], instrument.Expiry)
		}
		return response.SuccessResponse(c, responseData)

	} else {
		return response.SuccessResponse(c, instruments)
	}
}
