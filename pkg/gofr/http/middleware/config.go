package middleware

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gofr.dev/pkg/gofr/config"
)

func GetConfigs(c config.Config) map[string]string {
	middlewareConfigs := make(map[string]string)

	allowedCORSHeaders := []string{
		"ACCESS_CONTROL_ALLOW_ORIGIN",
		"ACCESS_CONTROL_ALLOW_HEADERS",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS",
		"ACCESS_CONTROL_EXPOSE_HEADERS",
		"ACCESS_CONTROL_MAX_AGE",
	}

	allowTracers := []string{
		"TRACE_EXPORTER",
		"TRACER_URL",
		"TRACER_HOST",
		"TRACER_PORT",
	}

	for _, v := range allowedCORSHeaders {
		if val := c.Get(v); val != "" {
			middlewareConfigs[convertHeaderNames(v)] = val
		}
	}

	for _, v := range allowTracers {
		if val := c.Get(v); val != "" {
			middlewareConfigs[v] = val
		}
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
