// Package handlers contains the handlers for the API
package handlers

import (
	"net/http"
	"regexp"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"gorm.io/gorm"
)

type OptionchainHandler struct {
	DB                 *gorm.DB
	InstrumentService  *service.InstrumentService
	OptionchainService *service.OptionchainService
}

func NewOptionchainHandler(db *gorm.DB) *OptionchainHandler {
	return &OptionchainHandler{
		DB:                 db,
		InstrumentService:  service.NewInstrumentService(db),
		OptionchainService: service.NewOptionchainService(db),
	}
}

// GetOptionChainNames returns a list of exchange:name for a given expiry
func (h *OptionchainHandler) GetOptionChainNames(c echo.Context) error {
	expiry := c.Param("expiry")
	if len(expiry) == 0 {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "No `expiry` provided")
	}

	// check if expiry is valid date
	_, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid `expiry` format")
	}

	ocNames, err := h.OptionchainService.GetOptionChainNames(expiry)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	var responseData []string
	if len(ocNames) == 0 {
		responseData = []string{expiry}
	} else {
		responseData = ocNames
	}
	return response.SuccessResponse(c, responseData)
}

// GetOptionChainInstruments returns a list of instruments for a given exchange, name and expiry
func (h *OptionchainHandler) GetOptionChainInstruments(c echo.Context) error {
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

	instruments, err := h.OptionchainService.GetOptionChainInstruments(exchange, name, expiry, details)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, instruments)

}
