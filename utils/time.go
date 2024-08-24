package utils

import "time"

// CurrentTime returns the current time formatted as "YYYY-MM-DD HH:MM:SS"
func CurrentTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
