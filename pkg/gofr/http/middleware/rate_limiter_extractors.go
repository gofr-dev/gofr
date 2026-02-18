package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const (
	// maxBodySize is the maximum size of request body that ExtractBody will read.
	// This prevents memory exhaustion attacks from large request bodies.
	maxBodySize = 1024 * 1024 // 1 MB
)

var (
	// ErrKeyNotFound is returned when the key cannot be extracted from the request.
	ErrKeyNotFound = errors.New("rate limit key not found")

	// ErrInvalidBody is returned when the request body cannot be parsed.
	ErrInvalidBody = errors.New("invalid request body")

	// ErrBodyTooLarge is returned when the request body exceeds the maximum allowed size.
	ErrBodyTooLarge = errors.New("request body too large")

	// ErrNoExtractors is returned when ExtractCombined is called with no extractors.
	ErrNoExtractors = errors.New("no extractors provided")
)

// KeyExtractor is a function that extracts a rate limiting key from an HTTP request.
// Returns the key string and an error if extraction fails.
//
// Security: When using KeyExtractor with user-controlled data (headers, params, body fields),
// attackers can create many unique rate limiter buckets by sending requests with random values,
// potentially consuming memory up to MaxKeys limit. Consider:
//   - Setting an appropriate MaxKeys value (default 100000)
//   - Using ExtractCombined with fallback to ExtractIP for additional protection
//   - Validating/sanitizing extracted keys if they come from untrusted sources
//   - Monitoring rate limiter memory usage in production
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
//
// Security Warning: Attackers can send requests with arbitrary header values to create
// many unique rate limiter buckets. Ensure MaxKeys is set appropriately.
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
//
// Security Warning: Attackers can send requests with arbitrary query parameters to create
// many unique rate limiter buckets. Ensure MaxKeys is set appropriately.
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
// Note: This reads the body (up to 1MB) and restores it for subsequent handlers.
// Bodies larger than 1MB will return ErrBodyTooLarge to prevent memory exhaustion attacks.
//
// Security Warning: Attackers can send requests with arbitrary body values to create
// many unique rate limiter buckets. Ensure MaxKeys is set appropriately.
func ExtractBody(field string) KeyExtractor {
	return func(r *http.Request) (string, error) {
		// Only work with JSON content
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			return "", ErrInvalidBody
		}

		// Limit body size to prevent memory exhaustion attacks
		limitedBody := io.LimitReader(r.Body, maxBodySize+1)
		body, err := io.ReadAll(limitedBody)
		
		// Close the original body immediately as we'll replace it
		originalBody := r.Body
		if closeErr := originalBody.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		
		if err != nil {
			return "", err
		}

		// Check if body exceeded the limit
		if len(body) > maxBodySize {
			return "", ErrBodyTooLarge
		}

		// Restore the body for subsequent handlers
		r.Body = io.NopCloser(bytes.NewReader(body))

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

// ExtractCombined creates a KeyExtractor that tries multiple extractors in order.
// Returns the first successful extraction or the last error encountered.
// Example: ExtractCombined(ExtractHeader("X-API-Key"), ExtractIP(false))
//
// Panics if no extractors are provided.
func ExtractCombined(extractors ...KeyExtractor) KeyExtractor {
	if len(extractors) == 0 {
		panic("ExtractCombined requires at least one extractor")
	}

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
