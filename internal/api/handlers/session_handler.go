// Package handlers contains the handlers for the API
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
)

// SessionHandler is the handler for the session API
type SessionHandler struct {
	service *service.SessionService
}

// NewSessionHandler creates a new handler for the session API
func NewSessionHandler(service *service.SessionService) *SessionHandler {
	return &SessionHandler{service: service}
}

// GenerateSession generates a new session for the given user
func (h *SessionHandler) GenerateSession(c echo.Context) error {
	// get the user_id, password, and totp_secret from the request
	userID := c.FormValue("user_id")
	password := c.FormValue("password")
	totpValue := c.FormValue("totp_value")
	totpSecret := c.FormValue("totp_secret")

	// check if all fields are present in the request
	if userID == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`user_id` is a required field")
	}
	if password == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`password` is a required field")
	}
	if totpValue == "" && totpSecret == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Either `totp_value` or `totp_secret` is required")
	}

	// generate the totp value, if top_secret is provided
	if totpSecret != "" {
		totpValueGenerated, err := h.service.GenerateTOTP(totpSecret)
		if err != nil {
			return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
		}
		totpValue = totpValueGenerated
	}

	// generate the session
	sessionData, err := h.service.GenerateSession(userID, password, totpValue)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthenticationException", err.Error())
	}

	return response.SuccessResponse(c, sessionData)
}

// GenerateTOTP generates a TOTP value for the given secret
func (h *SessionHandler) GenerateTOTP(c echo.Context) error {

	// get the totp_secret from the request
	totpSecret := c.FormValue("totp_secret")

	// check if the totp_secret is present in the request
	if totpSecret == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`totp_secret` is a required field")
	}

	// generate the totp value
	totpValue, err := h.service.GenerateTOTP(totpSecret)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, map[string]string{"totp_value": totpValue})
}

// CheckSessionValid checks if the given enctoken is valid
func (h *SessionHandler) CheckSessionValid(c echo.Context) error {
	// get the enctoken from the request
	enctoken := c.FormValue("enctoken")

	// check if the enctoken is present in the request
	if enctoken == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`enctoken` is a required field")
	}

	// check if the enctoken is valid
	isValid, err := h.service.CheckSessionValid(enctoken)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, map[string]bool{"is_valid": isValid})
}
