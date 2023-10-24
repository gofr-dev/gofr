package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"gofr.dev/pkg/errors"
)

// Header encapsulates the HTTP headers to be validated and checked.
type Header struct {
	Value         *http.Header
	IsBodyPresent bool
}

//nolint:gochecknoglobals // since we need to iterate through all the values provided by v3
var (
	xZopsmartTenantValues = map[string]bool{"good4more": true, "zopsmart": true}

	// key is the header name and the value is the default value of the header as per v3
	// headers with empty default values are mandatory headers
	requiredHeaders = map[string]string{
		"Accept-Language":   "en-US",
		"Content-Language":  "en-US",
		"Content-Type":      "application/json",
		"True-Client-Ip":    "",
		"X-Correlation-ID":  "",
		"X-Zopsmart-Tenant": "",
	}
)

// ValidateHeaders an HTTP middleware that checks and validates specified headers, and sets security-related response headers.
//
//nolint:gocognit // reduces readability
func ValidateHeaders(envHeaders string, logger logger) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ExemptPath(r) {
				inner.ServeHTTP(w, r)
				return
			}

			finalErr := errors.MultipleErrors{StatusCode: http.StatusBadRequest}

			var envSlc []string

			if envHeaders != "" {
				envSlc = strings.Split(envHeaders, ",")
			}

			errs := validateAllHeaders(r, envSlc)
			if errs != nil {
				finalErr.Errors = append(finalErr.Errors, errs...)
			}

			if len(finalErr.Errors) > 0 {
				ErrorResponse(w, r, logger, finalErr)
				return
			}

			// response header reported from AppScan result
			w.Header().Set("Strict-Transport-Security", "max-age=86400; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-XSS-Protection", "1")

			inner.ServeHTTP(w, r)
		})
	}
}

// ExemptPath checks if the path of an HTTP request matches specific patterns  used for metrics, health checks, or Swagger documentation
func ExemptPath(r *http.Request) bool {
	return strings.HasSuffix(r.URL.Path, "/metrics") ||
		strings.HasSuffix(r.URL.Path, "/.well-known/health-check") || strings.HasSuffix(r.URL.Path, "/.well-known/heartbeat") ||
		strings.HasSuffix(r.URL.Path, "/.well-known/openapi.json") || strings.Contains(r.URL.Path, "/swagger") ||
		strings.Contains(r.URL.Path, "/.well-known/swagger")
}

func createError(header, value string) *errors.Response {
	reason := fmt.Sprintf("Header %v is missing", header)

	if value != "" {
		reason = fmt.Sprintf("Header %v value is invalid", header)
	}

	now := time.Now()
	timeZone, _ := now.Zone()

	return &errors.Response{
		Code:   "BAD_REQUEST",
		Reason: reason,
		DateTime: errors.DateTime{
			Value:    now.UTC().Format(time.RFC3339),
			TimeZone: timeZone,
		},
	}
}

// isValidZopsmartTenant check if the value of `X-Zopsmart-Tenant` is one of the values in map
func isValidZopsmartTenant(val string) bool {
	val = strings.ToLower(val)
	if val == "" || !xZopsmartTenantValues[val] {
		return false
	}

	return true
}

// isValidTrueClientIP check if the value of "True-Client-Ip" is valid or not
func isValidTrueClientIP(headerVal string) bool {
	ip := net.ParseIP(headerVal)

	return ip != nil
}

// validateAllHeaders validates the mandatory headers as well as headers provided in config,
// it also sets the default value for headers as per v3.
// if header is not present in the request, and v3 doesn't provide a default value, error is returned
//
//nolint:gocognit,gocyclo // reduces readability
func validateAllHeaders(r *http.Request, envHeaders []string) (errs []error) {
	reqHeaders := r.Header
	// validate mandatory headers
	allHeaders := map[string]string{}
	for k, v := range requiredHeaders {
		allHeaders[k] = v
	}

	for _, v := range envHeaders {
		allHeaders[v] = ""
	}

	for k, defaultVal := range allHeaders {
		headerVal := reqHeaders.Get(k)

		// validate x-zopsmart-tenant from the list of values
		if k == "X-Zopsmart-Tenant" && !isValidZopsmartTenant(headerVal) {
			errs = append(errs, createError(k, headerVal))
			continue
		}

		// correlationId can be present in X-B3-TraceID as well
		if k == "X-Correlation-ID" && headerVal == "" {
			val := reqHeaders.Get("X-B3-TraceID")
			if val != "" {
				r.Header.Set("X-Correlation-ID", val)
				continue
			}

			errs = append(errs, createError(k, headerVal))

			continue
		}

		// for true client ip
		if k == "True-Client-Ip" && !isValidTrueClientIP(headerVal) {
			errs = append(errs, createError(k, headerVal))

			continue
		}

		// if header not present in the request, set the default value
		// if default value not present, throw an error
		if headerVal == "" {
			if defaultVal == "" {
				errs = append(errs, createError(k, headerVal))
			} else {
				r.Header.Set(k, defaultVal)
			}
		}
	}

	return errs
}
