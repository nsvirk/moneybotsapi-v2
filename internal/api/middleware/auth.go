package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"gorm.io/gorm"
)

// AuthMiddleware creates a new authorization middleware
func AuthMiddleware(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get the userId and enctoken from the authorization header
			enctoken, err := ExtractEnctokenFromAuthHeader(c)
			if err != nil {
				return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", err.Error())
			}

			// Verify the session
			sessionService := service.NewSessionService(db)
			userSession, err := sessionService.VerifySession(enctoken)
			if err != nil {
				return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", err.Error())
			}

			// Add session data to context for use in handlers
			c.Set("user_id", userSession.UserId)
			c.Set("enctoken", userSession.Enctoken)
			c.Set("user_session", userSession)

			// Get from the context to verify that the data was set
			// userID = c.Get("user_id").(string)
			// enctoken = c.Get("enctoken").(string)
			// userSession = c.Get("user_session").(*models.SessionModel)

			return next(c)
		}
	}
}

// ExtractEnctokenFromAuthHeader extracts the enctoken from the authorization header
func ExtractEnctokenFromAuthHeader(c echo.Context) (string, error) {
	// header format is <enctoken <enctoken>>
	auth := c.Request().Header.Get("Authorization")
	if auth == "" {
		return "", errors.New("missing authorization header")
	}
	// Split the authorization header into two parts on space
	partsToken := strings.SplitN(auth, " ", 2)
	if len(partsToken) != 2 {
		return "", errors.New("invalid authorization header format")
	}
	enctoken := partsToken[1]

	return enctoken, nil
}

// GetUserIdEnctokenFromEchoContext gets the userId and enctoken from the echo context
func GetUserIdEnctokenFromEchoContext(c echo.Context) (string, string, error) {
	userId, ok := c.Get("user_id").(string)
	if !ok {
		return "", "", errors.New("missing `user_id` in context")
	}
	enctoken, ok := c.Get("enctoken").(string)
	if !ok {
		return "", "", errors.New("missing `enctoken` in context")
	}
	return userId, enctoken, nil
}

// GetUserSessionFromEchoContext gets the user session from the echo context
func GetUserSessionFromEchoContext(c echo.Context) (*models.SessionModel, error) {
	userSession, ok := c.Get("user_session").(*models.SessionModel)
	if !ok {
		return nil, errors.New("missing `user_session` in context")
	}
	return userSession, nil
}
