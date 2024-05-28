package middleware

import (
	"strings"

	"gofr.dev/pkg/gofr/config"
)

func GetConfigs(c config.Config) map[string]string {
	middlewareConfigs := make(map[string]string)

	allowedCORSHeaders := []string{
		"ACCESS_CONTROL_ALLOW_ORIGIN",
		"ACCESS_CONTROL_ALLOW_HEADERS",
		"ACCESS_CONTROL_ALLOW_METHODS",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS",
		"ACCESS_CONTROL_EXPOSE_HEADERS",
		"ACCESS_CONTROL_MAX_AGE",
	}

	for _, v := range allowedCORSHeaders {
		if val := c.Get(v); val != "" {
			middlewareConfigs[convertHeaderNames(v)] = val
		}
	}

	return middlewareConfigs
}

func convertHeaderNames(header string) string {
	words := strings.Split(header, "_")

	for i, v := range words {
		words[i] = strings.Title(strings.ToLower(v))
	}

	return strings.Join(words, "-")
}
