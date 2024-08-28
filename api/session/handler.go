package session

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/shared/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GenerateSession(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid request body")
	}
	sessionData, err := h.service.GenerateSession(req)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthenticationException", err.Error())
	}

	return response.SuccessResponse(c, sessionData)
}

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
