/*
Package middleware provides set of middleware functions that can be used to authenticate and authorize
requests in HTTP server.It also supports handling CORS, propagating headers, integrating with New Relic APM, and enabling
distributed tracing using OpenTelemetry.
*/
package middleware

import (
	"net/http"
)

const (
	allowedHeaders = "Authorization, Content-Type, x-requested-with, true-client-ip, X-Correlation-ID"
	allowedMethods = "PUT, POST, GET, DELETE, OPTIONS, PATCH"
)

// CORS an HTTP middleware that sets headers based on the provided envHeaders configuration
func CORS(envHeaders map[string]string) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corsHeaderMappings := map[string]string{
				"ACCESS_CONTROL_ALLOW_HEADERS":     "Access-Control-Allow-Headers",
				"ACCESS_CONTROL_ALLOW_METHODS":     "Access-Control-Allow-Methods",
				"ACCESS_CONTROL_ALLOW_CREDENTIALS": "Access-Control-Allow-Credentials",
				"ACCESS_CONTROL_EXPOSE_HEADERS":    "Access-Control-Expose-Headers",
				"ACCESS_CONTROL_MAX_AGE":           "Access-Control-Max-Age",
				"ACCESS_CONTROL_ALLOW_ORIGIN":      "Access-Control-Allow-Origin",
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
		// If config is set, use that
		if val, ok := configHeaders[header]; ok && val != "" {
			validCORSHeadersAndValues[headerMappings[header]] = val
			continue
		}

		// If config is not set - for the three headers, set default value.
		switch header {
		case "ACCESS_CONTROL_ALLOW_HEADERS":
			validCORSHeadersAndValues[headerMappings[header]] = allowedHeaders
		case "ACCESS_CONTROL_ALLOW_METHODS":
			validCORSHeadersAndValues[headerMappings[header]] = allowedMethods
		}
	}

	val := validCORSHeadersAndValues[headerMappings["ACCESS_CONTROL_ALLOW_HEADERS"]]

	if val != allowedHeaders {
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
	}
}
