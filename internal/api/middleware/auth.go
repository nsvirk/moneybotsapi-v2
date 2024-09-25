package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/service"
	"github.com/nsvirk/moneybotsapi/pkg/utils/response"
	"gorm.io/gorm"
)

// AuthMiddleware creates a new authorization middleware
func AuthMiddleware(db *gorm.DB) echo.MiddlewareFunc {
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

			sessionService := service.NewSessionService(db)
			userSession, err := sessionService.VerifySession(userID, enctoken)
			if err != nil {
				return response.ErrorResponse(c, http.StatusUnauthorized, "AuthorizationException", "Invalid or expired session")
			}

			// Add session data to context for use in handlers
			c.Set("user_id", userSession.UserID)
			c.Set("enctoken", userSession.Enctoken)
			c.Set("user_session", userSession)

			// // Get from the context to verify that the data was set
			// userID = c.Get("user_id").(string)
			// enctoken = c.Get("enctoken").(string)
			// userSession = c.Get("user_session").(*models.SessionModel)

			return next(c)
		}
	}
}
