// Package session handles the API for session operations
package session

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/services/session"
	"github.com/nsvirk/moneybotsapi/shared/response"
)

// Handler is the handler for the session API
type Handler struct {
	service *session.SessionService
}

// NewHandler creates a new handler for the session API
func NewHandler(service *session.SessionService) *Handler {
	return &Handler{service: service}
}

// GenerateSession generates a new session for the given user
func (h *Handler) GenerateSession(c echo.Context) error {
	var req struct {
		UserID     string `json:"user_id"`
		Password   string `json:"password"`
		TOTPSecret string `json:"totp_secret"`
	}
	if err := c.Bind(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid request body")
	}
	sessionData, err := h.service.GenerateSession(req.UserID, req.Password, req.TOTPSecret)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthenticationException", err.Error())
	}

	return response.SuccessResponse(c, sessionData)
}

// GenerateTOTP generates a TOTP value for the given secret
func (h *Handler) GenerateTOTP(c echo.Context) error {
	var req struct {
		TOTPSecret string `json:"totp_secret"`
	}
	if err := c.Bind(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid request body")
	}

	totpValue, err := h.service.GenerateTOTP(req.TOTPSecret)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, map[string]string{"totp_value": totpValue})
}

// CheckSessionValid checks if the given enctoken is valid
func (h *Handler) CheckSessionValid(c echo.Context) error {
	var req struct {
		Enctoken string `json:"enctoken"`
	}
	if err := c.Bind(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid request body")
	}

	isValid, err := h.service.CheckSessionValid(req.Enctoken)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, map[string]bool{"is_valid": isValid})
}
