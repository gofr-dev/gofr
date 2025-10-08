package service

import (
	"context"
	"net/http"
	"reflect"
	"strings"
)

// rateLimiter provides unified rate limiting for HTTP clients.
type rateLimiter struct {
	config RateLimiterConfig
	store  RateLimiterStore
	HTTP   // Embedded HTTP service
}

// NewRateLimiter creates a new unified rate limiter.
func NewRateLimiter(config RateLimiterConfig, h HTTP) HTTP {
	_ = config.Validate() // Apply defaults even if validation fails

	return createRateLimiter(config, h)
}

// NewRateLimiterWithValidation creates a rate limiter with validation
func NewRateLimiterWithValidation(config RateLimiterConfig, h HTTP) (HTTP, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create with validated config
	return createRateLimiter(config, h), nil
}

func createRateLimiter(config RateLimiterConfig, h HTTP) HTTP {
	rl := &rateLimiter{
		config: config,
		store:  config.Store,
		HTTP:   h,
	}

	ctx := context.Background()
	rl.store.StartCleanup(ctx)

	return rl
}

// AddOption allows RateLimiterConfig to be used as a service.Options.
func (cfg *RateLimiterConfig) AddOption(h HTTP) HTTP {
	// Assume cfg is already validated via constructor
	if cfg.Store == nil {
		cfg.Store = NewLocalRateLimiterStore()
	}

	return NewRateLimiter(*cfg, h)
}

// AddOptionWithValidation implements the AddOptionWithValidation interface
func (r *RateLimiterConfig) AddOptionWithValidation(h HTTP) (HTTP, error) {
	if r == nil {
		return h, nil
	}

	return NewRateLimiterWithValidation(*r, h)
}

// Keep the existing AddOption method for backward compatibility

// buildFullURL constructs an absolute URL by combining the base service URL with the given path.
func (rl *rateLimiter) buildFullURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	// Get base URL from embedded HTTP service
	httpSvcImpl, ok := rl.HTTP.(*httpService)
	if !ok {
		return path
	}

	base := strings.TrimRight(httpSvcImpl.url, "/")
	if base == "" {
		return path
	}

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return base + path
}

// checkRateLimit performs rate limit check using the configured store.
func (rl *rateLimiter) checkRateLimit(req *http.Request) error {
	if isCircuitOpen(rl.HTTP) {
		// Skip rate limiting if circuit is open
		return nil
	}

	serviceKey := rl.config.KeyFunc(req)

	allowed, retryAfter, err := rl.store.Allow(req.Context(), serviceKey, rl.config)
	if err != nil {
		return nil // Fail open
	}

	if !allowed {
		return &RateLimitError{ServiceKey: serviceKey, RetryAfter: retryAfter}
	}

	return nil
}

// isCircuitOpen checks if there's a circuit breaker in the chain and if it's open
func isCircuitOpen(h HTTP) bool {
	// Direct check
	if cb, ok := h.(*circuitBreaker); ok {
		return cb.isOpen()
	}

	// Use reflection to check for embedded HTTP field
	v := reflect.ValueOf(h)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
		if v.Kind() == reflect.Struct {
			// Look for HTTP field that might contain a circuit breaker
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				if field.Type().String() == "service.HTTP" && !field.IsNil() {
					// Recursively check if the HTTP field has a circuit breaker
					if httpField, ok := field.Interface().(HTTP); ok {
						return isCircuitOpen(httpField)
					}
				}
			}
		}
	}

	return false
}

// Get performs rate-limited HTTP GET request.
func (rl *rateLimiter) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Get(ctx, path, queryParams)
}

// GetWithHeaders performs rate-limited HTTP GET request with custom headers.
func (rl *rateLimiter) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

// Post performs rate-limited HTTP POST request.
func (rl *rateLimiter) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Post(ctx, path, queryParams, body)
}

// PostWithHeaders performs rate-limited HTTP POST request with custom headers.
func (rl *rateLimiter) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

// Put performs rate-limited HTTP PUT request.
func (rl *rateLimiter) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Put(ctx, path, queryParams, body)
}

// PutWithHeaders performs rate-limited HTTP PUT request with custom headers.
func (rl *rateLimiter) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

// Patch performs rate-limited HTTP PATCH request.
func (rl *rateLimiter) Patch(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Patch(ctx, path, queryParams, body)
}

// PatchWithHeaders performs rate-limited HTTP PATCH request with custom headers.
func (rl *rateLimiter) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

// Delete performs rate-limited HTTP DELETE request.
func (rl *rateLimiter) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Delete(ctx, path, body)
}

// DeleteWithHeaders performs rate-limited HTTP DELETE request with custom headers.
func (rl *rateLimiter) DeleteWithHeaders(ctx context.Context, path string, body []byte,
	headers map[string]string) (*http.Response, error) {
	fullURL := rl.buildFullURL(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}
