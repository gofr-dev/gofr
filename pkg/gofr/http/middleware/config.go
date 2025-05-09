package middleware

import (
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/service"
)

type Config struct {
	CorsHeaders map[string]string
	LogProbes   LogProbes
}

type LogProbes struct {
	Disabled bool
	Paths    []string
}

func GetConfigs(c config.Config) Config {
	middlewareConfigs := Config{
		CorsHeaders: make(map[string]string),
	}

	allowedCORSHeaders := []string{
		"ACCESS_CONTROL_ALLOW_ORIGIN",
		"ACCESS_CONTROL_ALLOW_HEADERS",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS",
		"ACCESS_CONTROL_EXPOSE_HEADERS",
		"ACCESS_CONTROL_MAX_AGE",
	}

	for _, v := range allowedCORSHeaders {
		if val := c.Get(v); val != "" {
			middlewareConfigs.CorsHeaders[convertHeaderNames(v)] = val
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

	return middlewareConfigs
}

func convertHeaderNames(header string) string {
	words := strings.Split(header, "_")
	titleCaser := cases.Title(language.Und)

	for i, v := range words {
		words[i] = titleCaser.String(strings.ToLower(v))
	}

	return strings.Join(words, "-")
}
