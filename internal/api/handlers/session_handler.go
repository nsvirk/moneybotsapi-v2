// Package handlers contains the handlers for the API
package handlers

import (
	"net/http"
	"time"

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
	userid := c.FormValue("user_id")
	password := c.FormValue("password")
	totpValue := c.FormValue("totp_value")
	totpSecret := c.FormValue("totp_secret")

	// check if all fields are present in the request
	if userid == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`user_id` is required")
	}
	if password == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`password` is required")
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
	sessionData, err := h.service.GenerateSession(userid, password, totpValue)
	if err != nil {
		return response.ErrorResponse(c, http.StatusUnauthorized, "AuthenticationException", err.Error())
	}

	// set the cookies
	// Cookie 1: user_id
	useridCookie := &http.Cookie{
		Name:     "user_id",
		Value:    userid,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}
	c.SetCookie(useridCookie)

	// Cookie 2: public_token
	publicTokenCookie := &http.Cookie{
		Name:   "public_token",
		Value:  sessionData.PublicToken,
		Domain: ".zerodha.com",
		Path:   "/",
		Secure: true,
	}
	c.SetCookie(publicTokenCookie)

	// Cookie 3: enctoken
	enctokenCookie := &http.Cookie{
		Name:     "enctoken",
		Value:    sessionData.Enctoken,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}
	c.SetCookie(enctokenCookie)

	// Cookie 4: kf_session
	kfSessionCookie := &http.Cookie{
		Name:     "kf_session",
		Value:    sessionData.KfSession,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	}
	c.SetCookie(kfSessionCookie)

	return response.SuccessResponse(c, sessionData)
}

// GenerateTOTP generates a TOTP value for the given secret
func (h *SessionHandler) GenerateTOTP(c echo.Context) error {

	// get the user_id and totp_secret from the request
	userid := c.FormValue("user_id")
	if userid == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`user_id` is required")
	}
	// get the totp_secret from the request
	totpSecret := c.FormValue("totp_secret")
	if totpSecret == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`totp_secret` is required")
	}

	// generate the totp value
	totpValue, err := h.service.GenerateTOTP(totpSecret)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}

	return response.SuccessResponse(c, map[string]string{"user_id": userid, "totp_value": totpValue})
}

// DeleteSession deletes the session for the given user
func (h *SessionHandler) DeleteSession(c echo.Context) error {
	// get the user_id from the request
	userId := c.FormValue("user_id")
	if userId == "" {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "`user_id` is a required field")
	}

	// delete the session
	err := h.service.DeleteSession(userId)
	if err != nil {
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerException", err.Error())
	}
	// Clear user_id cookie
	c.SetCookie(&http.Cookie{
		Name:     "user_id",
		Value:    "",
		Path:     "/",
		Domain:   "",
		Expires:  time.Now().Add(-1 * time.Hour),
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	// Clear public_token cookie
	c.SetCookie(&http.Cookie{
		Name:     "public_token",
		Value:    "",
		Path:     "/",
		Domain:   ".zerodha.com",
		Expires:  time.Now().Add(-1 * time.Hour),
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	// Clear enctoken cookie
	c.SetCookie(&http.Cookie{
		Name:     "enctoken",
		Value:    "",
		Path:     "/",
		Domain:   "",
		Expires:  time.Now().Add(-1 * time.Hour),
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	// Clear the kf_session cookie
	c.SetCookie(&http.Cookie{
		Name:     "kf_session",
		Value:    "",
		Path:     "/",
		Domain:   "",
		Expires:  time.Now().Add(-1 * time.Hour),
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	return response.SuccessResponse(c, map[string]string{"user_id": userId})
}
