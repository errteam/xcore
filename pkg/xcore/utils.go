package xcore

import "time"

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// Context keys for package-wide use
const (
	// ContextKeyRequestID is the context key for request ID
	ContextKeyRequestID contextKey = "request_id"
	// ContextKeyStartTime is the context key for request start time
	ContextKeyStartTime contextKey = "start_time"
	// ContextKeyLogger is the context key for logger
	ContextKeyLogger contextKey = "logger"
)

// joinHeader joins a slice of strings with commas for HTTP headers.
func joinHeader(headers []string) string {
	result := ""
	for i, h := range headers {
		if i > 0 {
			result += ", "
		}
		result += h
	}
	return result
}

// parseDuration parses a duration string, returning a default if empty or invalid.
func parseDuration(d string) time.Duration {
	if d == "" {
		return 0
	}
	duration, err := time.ParseDuration(d)
	if err != nil {
		return 0
	}
	return duration
}

// CalculateTotalPages calculates the total number of pages based on total items and items per page.
func CalculateTotalPages(total, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	return pages
}
