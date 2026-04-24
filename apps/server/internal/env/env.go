package env

import (
	"os"
	"strings"
)

// String returns the trimmed env value when set, otherwise fallback.
func String(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return fallback
}

// Bool parses key as a boolean. The second return is false if the variable is unset or not a
// recognized true/false string (empty, unknown value).
func Bool(key string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}
