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
	allowedHeaders = "Authorization, Content-Type, x-requested-with, origin, true-client-ip, X-Correlation-ID"
	allowedMethods = "PUT, POST, GET, DELETE, OPTIONS, PATCH"
)

// CORS an HTTP middleware that sets headers based on the provided envHeaders configuration
func CORS(envHeaders map[string]string) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corsHeadersConfig := getValidCORSHeaders(envHeaders)
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
func getValidCORSHeaders(envHeaders map[string]string) map[string]string {
	validCORSHeadersAndValues := make(map[string]string)

	for _, header := range AllowedCORSHeader() {
		// If config is set, use that
		if val, ok := envHeaders[header]; ok && val != "" {
			validCORSHeadersAndValues[header] = val
			continue
		}

		// If config is not set - for the three headers, set default value.
		switch header {
		case "Access-Control-Allow-Origin":
			validCORSHeadersAndValues[header] = "*"
		case "Access-Control-Allow-Headers":
			validCORSHeadersAndValues[header] = allowedHeaders
		case "Access-Control-Allow-Methods":
			validCORSHeadersAndValues[header] = allowedMethods
		}
	}

	val := validCORSHeadersAndValues["Access-Control-Allow-Headers"]

	if val != allowedHeaders {
		validCORSHeadersAndValues["Access-Control-Allow-Headers"] = allowedHeaders + ", " + val
	}

	return validCORSHeadersAndValues
}

// AllowedCORSHeader returns the HTTP headers used for CORS configuration in web applications.
func AllowedCORSHeader() []string {
	return []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Headers",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Credentials",
		"Access-Control-Expose-Headers",
		"Access-Control-Max-Age",
	}
}
