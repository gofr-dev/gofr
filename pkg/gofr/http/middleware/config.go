package middleware

import (
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/service"
)

// CORS configuration keys whose values have a defined syntax, and are therefore
// validated before being emitted as response headers.
const (
	keyAccessControlMaxAge           = "ACCESS_CONTROL_MAX_AGE"
	keyAccessControlAllowCredentials = "ACCESS_CONTROL_ALLOW_CREDENTIALS"
)

type Config struct {
	CorsHeaders map[string]string
	LogProbes   LogProbes
}

type LogProbes struct {
	Disabled bool
	Paths    []string
}

// configLogger is the logging surface GetConfigs needs to report a misconfigured
// value. The container's logger, held at the call site, satisfies it.
type configLogger interface {
	Warnf(format string, args ...any)
}

// GetConfigs reads the middleware configuration from c. CORS values with a defined
// syntax are validated; an invalid one is dropped and reported through logger
// instead of being emitted as a malformed response header.
func GetConfigs(c config.Config, logger configLogger) Config {
	middlewareConfigs := Config{
		CorsHeaders: make(map[string]string),
	}

	allowedCORSHeaders := []string{
		"ACCESS_CONTROL_ALLOW_ORIGIN",
		"ACCESS_CONTROL_ALLOW_METHODS",
		"ACCESS_CONTROL_ALLOW_HEADERS",
		keyAccessControlAllowCredentials,
		"ACCESS_CONTROL_EXPOSE_HEADERS",
		keyAccessControlMaxAge,
	}

	for _, v := range allowedCORSHeaders {
		val := c.Get(v)
		if val == "" || !isValidCORSValue(v, val, logger) {
			continue
		}

		middlewareConfigs.CorsHeaders[convertHeaderNames(v)] = val
	}

	// Config values for Log Probes
	logDisableProbes := c.GetOrDefault("LOG_DISABLE_PROBES", "false")
	middlewareConfigs.LogProbes.Paths = []string{service.HealthPath, service.AlivePath}

	// Convert the string value to a boolean
	value, err := strconv.ParseBool(logDisableProbes)
	if err == nil {
		middlewareConfigs.LogProbes.Disabled = value
	}

	return middlewareConfigs
}

// isValidCORSValue reports whether val is usable for the given CORS configuration
// key. A browser discards a malformed CORS header and falls back to its own
// default, so an invalid value is dropped and reported rather than sent — left in
// place it is invisible in the logs and looks present in the response. Keys
// without a defined value syntax are accepted unchanged.
func isValidCORSValue(key, val string, logger configLogger) bool {
	var expected string

	switch key {
	case keyAccessControlMaxAge:
		if seconds, err := strconv.Atoi(val); err == nil && seconds >= 0 {
			return true
		}

		expected = "a non-negative number of seconds"
	case keyAccessControlAllowCredentials:
		if _, err := strconv.ParseBool(val); err == nil {
			return true
		}

		expected = "a boolean"
	default:
		return true
	}

	if logger != nil {
		logger.Warnf("invalid value %q for config %s, expected %s: dropping the header", val, key, expected)
	}

	return false
}

func convertHeaderNames(header string) string {
	words := strings.Split(header, "_")
	titleCaser := cases.Title(language.Und)

	for i, v := range words {
		words[i] = titleCaser.String(strings.ToLower(v))
	}

	return strings.Join(words, "-")
}
