package middleware

import (
	"net/http"
	"strings"
)

const (
	allowedMethods = "PUT, POST, GET, DELETE, OPTIONS, PATCH"
	allowedHeaders = "Authorization, Content-Type, x-requested-with, true-client-ip, X-Correlation-ID"

	AccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	AccessControlAllowMethods     = "Access-Control-Allow-Methods"
	AccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	AccessControlExposeHeaders    = "Access-Control-Expose-Headers"
	AccessControlMaxAge           = "Access-Control-Max-Age"
	AccessControlAllowOrigin      = "Access-Control-Allow-Origin"
)

// CORS is an HTTP middleware that sets headers based on the provided envHeaders configuration
func CORS(envHeaders map[string]string) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corsHeaderMappings := map[string]string{
				"ACCESS_CONTROL_ALLOW_HEADERS":     AccessControlAllowHeaders,
				"ACCESS_CONTROL_ALLOW_METHODS":     AccessControlAllowMethods,
				"ACCESS_CONTROL_ALLOW_CREDENTIALS": AccessControlAllowCredentials,
				"ACCESS_CONTROL_EXPOSE_HEADERS":    AccessControlExposeHeaders,
				"ACCESS_CONTROL_MAX_AGE":           AccessControlMaxAge,
				"ACCESS_CONTROL_ALLOW_ORIGIN":      AccessControlAllowOrigin,
			}

			corsHeadersConfig := getValidCORSHeaders(envHeaders, corsHeaderMappings)
			for k, v := range corsHeadersConfig {
				w.Header().Set(k, v)
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

// getValidCORSHeaders returns a validated map of CORS headers.
// values specified in env are present in envHeaders
func getValidCORSHeaders(configHeaders, headerMappings map[string]string) map[string]string {
	validCORSHeadersAndValues := make(map[string]string)

	for _, header := range AllowedCORSHeader() {
		convertedHeader := convertToUpper(header)

		// If config is set, use that
		if val, ok := configHeaders[header]; ok && val != "" {
			if _, ok := validCORSHeadersAndValues[convertedHeader]; !ok {
				validCORSHeadersAndValues[headerMappings[convertedHeader]] = val
				continue
			}
		}

		// If config is not set for these two headers, set default value.
		switch convertedHeader {
		case "ACCESS_CONTROL_ALLOW_HEADERS":
			if _, ok := validCORSHeadersAndValues[headerMappings[convertedHeader]]; !ok {
				validCORSHeadersAndValues[headerMappings[convertedHeader]] = allowedHeaders
			}

		case "ACCESS_CONTROL_ALLOW_METHODS":
			if _, ok := validCORSHeadersAndValues[headerMappings[convertedHeader]]; !ok {
				validCORSHeadersAndValues[headerMappings[convertedHeader]] = allowedMethods
			}
		}
	}

	val, _ := validCORSHeadersAndValues[headerMappings["ACCESS_CONTROL_ALLOW_HEADERS"]]

	if val != allowedHeaders && val != "" {
		validCORSHeadersAndValues[headerMappings["ACCESS_CONTROL_ALLOW_HEADERS"]] = allowedHeaders + ", " + val
	}

	return validCORSHeadersAndValues
}

// AllowedCORSHeader returns the HTTP headers used for CORS configuration in web applications.
func AllowedCORSHeader() []string {
	return []string{
		"ACCESS_CONTROL_ALLOW_HEADERS",
		"ACCESS_CONTROL_ALLOW_METHODS",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS",
		"ACCESS_CONTROL_EXPOSE_HEADERS",
		"ACCESS_CONTROL_MAX_AGE",
		"ACCESS_CONTROL_ALLOW_ORIGIN",
		AccessControlAllowHeaders,
		AccessControlAllowMethods,
		AccessControlAllowCredentials,
		AccessControlExposeHeaders,
		AccessControlMaxAge,
		AccessControlAllowOrigin,
	}
}

func convertToUpper(input string) string {
	if strings.Contains(input, "_") {
		return input
	} else {
		// Separate words based on hyphens
		words := strings.Split(input, "-")

		// Capitalize each word
		for i := range words {
			words[i] = strings.ToUpper(words[i])
		}

		// Join words using underscores
		output := strings.Join(words, "_")

		return output
	}
}
