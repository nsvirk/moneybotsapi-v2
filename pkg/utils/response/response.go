// Package response contains response utility functions and types
package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Response represents the standard API response structure
type Response struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	ErrorType string      `json:"error_type,omitempty"`
	Message   string      `json:"message,omitempty"`
}

// SuccessResponse sends a successful JSON response
func SuccessResponse(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, Response{
		Status: "success",
		Data:   data,
	})
}

// ErrorResponse sends an error JSON response
func ErrorResponse(c echo.Context, httpStatus int, errorType, message string) error {
	return c.JSON(httpStatus, Response{
		Status:    "error",
		ErrorType: errorType,
		Message:   message,
	})
}
