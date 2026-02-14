package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

var (
	// ErrKeyNotFound is returned when the key cannot be extracted from the request.
	ErrKeyNotFound = errors.New("rate limit key not found")

	// ErrInvalidBody is returned when the request body cannot be parsed.
	ErrInvalidBody = errors.New("invalid request body")
)

// KeyExtractor is a function that extracts a rate limiting key from an HTTP request.
// Returns the key string and an error if extraction fails.
type KeyExtractor func(*http.Request) (string, error)

// ExtractIP extracts the client IP address from the request.
// This is the default behavior when PerIP=true.
func ExtractIP(trustProxies bool) KeyExtractor {
	return func(r *http.Request) (string, error) {
		ip := getIP(r, trustProxies)
		if ip == "" {
			return "unknown", nil
		}
		return ip, nil
	}
}

// ExtractHeader creates a KeyExtractor that extracts a value from request headers.
// Example: ExtractHeader("X-API-Key") for API key rate limiting.
func ExtractHeader(name string) KeyExtractor {
	return func(r *http.Request) (string, error) {
		value := r.Header.Get(name)
		if value == "" {
			return "", ErrKeyNotFound
		}
		return value, nil
	}
}

// ExtractParam creates a KeyExtractor that extracts a value from URL query parameters.
// Example: ExtractParam("user_id") for per-user rate limiting.
func ExtractParam(name string) KeyExtractor {
	return func(r *http.Request) (string, error) {
		value := r.URL.Query().Get(name)
		if value == "" {
			return "", ErrKeyNotFound
		}
		return value, nil
	}
}

// ExtractBody creates a KeyExtractor that extracts a value from the JSON request body.
// Example: ExtractBody("email") for login attempt rate limiting.
// Note: This reads the body and restores it for subsequent handlers.
func ExtractBody(field string) KeyExtractor {
	return func(r *http.Request) (string, error) {
		// Only work with JSON content
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			return "", ErrInvalidBody
		}

		// Read the body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		defer r.Body.Close()

		// Restore the body for subsequent handlers
		r.Body = io.NopCloser(strings.NewReader(string(body)))

		// Parse JSON
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return "", ErrInvalidBody
		}

		// Extract field
		value, ok := data[field]
		if !ok {
			return "", ErrKeyNotFound
		}

		// Convert to string
		str, ok := value.(string)
		if !ok {
			return "", ErrKeyNotFound
		}

		if str == "" {
			return "", ErrKeyNotFound
		}

		return str, nil
	}
}

// ExtractPathParam creates a KeyExtractor that extracts a value from URL path parameters.
// This requires integration with the router to extract path params.
// For now, it falls back to checking query params as path params aren't directly accessible.
func ExtractPathParam(name string) KeyExtractor {
	return ExtractParam(name)
}

// ExtractCombined creates a KeyExtractor that tries multiple extractors in order.
// Returns the first successful extraction or the last error encountered.
// Example: ExtractCombined(ExtractHeader("X-API-Key"), ExtractIP(false))
func ExtractCombined(extractors ...KeyExtractor) KeyExtractor {
	return func(r *http.Request) (string, error) {
		var lastErr error
		for _, extractor := range extractors {
			key, err := extractor(r)
			if err == nil && key != "" {
				return key, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return "", lastErr
		}
		return "", ErrKeyNotFound
	}
}

// ExtractStatic creates a KeyExtractor that always returns a fixed key.
// Useful for global rate limiting.
func ExtractStatic(key string) KeyExtractor {
	return func(r *http.Request) (string, error) {
		return key, nil
	}
}
