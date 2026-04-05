package env

import (
	"os"
	"strings"
)

// Bool parses key as a boolean. The second return is false if the variable is unset or not a
// recognized true/false string (empty, unknown value).
func Bool(key string) (value bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

// String returns the trimmed value of key, or fallback if unset or empty after trimming.
func String(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
