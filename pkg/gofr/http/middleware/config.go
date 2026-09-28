package middleware

import (
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/service"
)

// CORS configuration keys. MaxAge and AllowCredentials have a defined value syntax,
// and are therefore validated before being emitted as response headers.
const (
	keyAccessControlAllowOrigin      = "ACCESS_CONTROL_ALLOW_ORIGIN"
	keyAccessControlMaxAge           = "ACCESS_CONTROL_MAX_AGE"
	keyAccessControlAllowCredentials = "ACCESS_CONTROL_ALLOW_CREDENTIALS"
)

// allowCredentialsTrue is the only value the Fetch standard recognizes for
// Access-Control-Allow-Credentials.
const allowCredentialsTrue = "true"

type Config struct {
	CorsHeaders map[string]string
	LogProbes   LogProbes

	// MetricsCardinalityLimit is the meter provider's effective per-instrument
	// datapoint ceiling, from container.MetricsCardinalityLimit so the two cannot
	// disagree. Zero or negative means unlimited. The Metrics middleware sizes its
	// caller-controlled label budget from it; see WithCardinalityLimit.
	MetricsCardinalityLimit int
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
// syntax are validated: one a browser would read the same way is rewritten to its
// canonical form, and one it cannot read is dropped. Either is reported through the
// optional logger instead of being emitted as a malformed response header.
func GetConfigs(c config.Config, logger ...configLogger) Config {
	middlewareConfigs := Config{
		CorsHeaders: make(map[string]string),
	}

	var warnLogger configLogger
	if len(logger) > 0 {
		warnLogger = logger[0]
	}

	allowedCORSHeaders := []string{
		keyAccessControlAllowOrigin,
		"ACCESS_CONTROL_ALLOW_METHODS",
		"ACCESS_CONTROL_ALLOW_HEADERS",
		keyAccessControlAllowCredentials,
		"ACCESS_CONTROL_EXPOSE_HEADERS",
		keyAccessControlMaxAge,
	}

	for _, v := range allowedCORSHeaders {
		val := c.Get(v)
		if val == "" {
			continue
		}

		if headerVal, ok := corsHeaderValue(v, val, warnLogger); ok {
			middlewareConfigs.CorsHeaders[convertHeaderNames(v)] = headerVal
		}
	}

	// Config values for Log Probes
	logDisableProbes := c.GetOrDefault("LOG_DISABLE_PROBES", "false")
	middlewareConfigs.LogProbes.Paths = []string{service.HealthPath, service.AlivePath}

	// Convert the string value to a boolean
	value, err := strconv.ParseBool(logDisableProbes)
	if err == nil {
		middlewareConfigs.LogProbes.Disabled = value
	}

	middlewareConfigs.MetricsCardinalityLimit = container.MetricsCardinalityLimit(c)

	return middlewareConfigs
}

// corsHeaderValue returns the value to send as the response header for the given
// CORS configuration key, and whether to send it at all. A browser discards a
// malformed CORS header, so a value it cannot read is dropped and reported rather
// than sent — left in place it is invisible in the logs and looks present in the
// response. Keys without a defined value syntax are emitted unchanged.
func corsHeaderValue(key, val string, logger configLogger) (string, bool) {
	var expected string

	switch key {
	case keyAccessControlMaxAge:
		if maxAge, ok := canonicalMaxAge(val, logger); ok {
			return maxAge, true
		}

		expected = "a non-negative number of seconds"
	case keyAccessControlAllowCredentials:
		// The Fetch standard matches this header against the literal "true", so the other
		// spellings strconv.ParseBool accepts (1, t, TRUE) are discarded by the browser.
		if val == allowCredentialsTrue {
			return val, true
		}

		// "false" is an explicit opt-out rather than a mistake, so it is not reported:
		// omitting the header is exactly what the browser does with that value anyway.
		if val == "false" {
			return "", false
		}

		expected = `exactly "true" or "false"`
	default:
		return val, true
	}

	if logger != nil {
		logger.Warnf("invalid value %q for config %s, expected %s: dropping the header", val, key, expected)
	}

	return "", false
}

// canonicalMaxAge rewrites an Access-Control-Max-Age value to the canonical decimal
// form the Fetch standard defines, preserving what browsers already make of it.
// Browsers read "+600" and "0600" as 600 seconds, and a negative value as "do not
// cache the preflight" — which is what 0 means — so dropping any of them would
// quietly replace the configured behavior with the browser's 5 second default.
// A value that is not an integer at all is not rewritten, and reports false.
func canonicalMaxAge(val string, logger configLogger) (string, bool) {
	seconds, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return "", false
	}

	canonical := strconv.FormatInt(max(seconds, 0), 10)
	if canonical != val && logger != nil {
		logger.Warnf("value %q for config %s is not in canonical form: sending %q instead",
			val, keyAccessControlMaxAge, canonical)
	}

	return canonical, true
}

func convertHeaderNames(header string) string {
	words := strings.Split(header, "_")
	titleCaser := cases.Title(language.Und)

	for i, v := range words {
		words[i] = titleCaser.String(strings.ToLower(v))
	}

	return strings.Join(words, "-")
}
