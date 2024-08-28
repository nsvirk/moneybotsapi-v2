package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/api/session"
	"github.com/nsvirk/moneybotsapi/shared/response"
)

// AuthMiddleware creates a new authorization middleware
func AuthMiddleware(sessionService *session.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if auth == "" {
				return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", "Missing Authorization header")
			}

			parts := strings.SplitN(auth, ":", 2)
			if len(parts) != 2 {
				return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", "Invalid Authorization header format")
			}

			userID, enctoken := parts[0], parts[1]

			userSession, err := sessionService.VerifySession(userID, enctoken)
			if err != nil {
				return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", "Invalid or expired session")
			}

			// Add session data to context for use in handlers
			c.Set("userSession", userSession)

			return next(c)
		}
	}
}
