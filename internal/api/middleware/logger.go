// Package middleware provides the middleware for the Echo instance
package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// SetupLoggerMiddleware configures and adds middleware to the Echo instance
func SetupLoggerMiddleware(e *echo.Echo) {
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${time_rfc3339}: ip=${remote_ip}, req=${method}, uri=${uri}, status=${status}, error=${error}, latency=${latency_human}\n",
	}))
	e.Use(middleware.Recover())
}
