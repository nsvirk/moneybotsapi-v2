package stream

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/shared/response"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		service: NewService(db),
	}
}

type RequestBody struct {
	Instruments []string `json:"instruments"`
}

func (h *Handler) StreamTickerData(c echo.Context) error {
	userId, enctoken, err := extractAuthInfo(c)
	if err != nil {
		return err
	}

	var req RequestBody
	if err := c.Bind(&req); err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "InputException", "Invalid request body")
	}

	ctx := c.Request().Context()
	errChan := make(chan error, 1)

	go h.service.RunTickerStream(ctx, c, userId, enctoken, req.Instruments, errChan)

	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return response.ErrorResponse(c, http.StatusInternalServerError, "ServerError", fmt.Sprintf("Ticker error: %v", err))
	}
}

func extractAuthInfo(c echo.Context) (string, string, error) {
	auth := c.Request().Header.Get("Authorization")
	userId, enctoken, found := strings.Cut(auth, ":")
	if !found {
		return "", "", response.ErrorResponse(c, http.StatusUnauthorized, "InputException", "Invalid authorization header")
	}
	return userId, enctoken, nil
}
