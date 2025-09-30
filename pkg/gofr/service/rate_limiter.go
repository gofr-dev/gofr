package service

import (
	"context"
	"net/http"
	"strings"
)

// rateLimiter provides unified rate limiting for HTTP clients.
type rateLimiter struct {
	config  RateLimiterConfig
	store   RateLimiterStore
	logger  Logger
	metrics Metrics
	HTTP    // Embedded HTTP service
}

// NewRateLimiter creates a new unified rate limiter.
func NewRateLimiter(config RateLimiterConfig, h HTTP) HTTP {
	httpSvc := h.(*httpService)

	rl := &rateLimiter{
		config:  config,
		store:   config.Store,
		logger:  httpSvc.Logger,
		metrics: httpSvc.Metrics,
		HTTP:    h,
	}

	// Start cleanup routine
	ctx := context.Background()
	rl.store.StartCleanup(ctx, rl.logger)

	return rl
}

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
	serviceKey := rl.config.KeyFunc(req)
	allowed, retryAfter, err := rl.store.Allow(req.Context(), serviceKey, rl.config)

	// Update metrics
	rl.updateRateLimiterMetrics(req.Context(), serviceKey, allowed, err)

	if err != nil {
		rl.logger.Log("Rate limiter store error, allowing request", "error", err)

		return nil // Fail open
	}

	if !allowed {
		rl.logger.Debug("Rate limit exceeded", "service", serviceKey, "rate", rl.config.RequestsPerSecond(),
			"burst", rl.config.Burst, "retry_after", retryAfter)

		return &RateLimitError{ServiceKey: serviceKey, RetryAfter: retryAfter}
	}

	return nil
}

// updateRateLimiterMetrics updates metrics for rate limiting operations.
func (rl *rateLimiter) updateRateLimiterMetrics(ctx context.Context, serviceKey string, allowed bool, err error) {
	if rl.metrics == nil {
		return
	}

	rl.metrics.IncrementCounter(ctx, "app_rate_limiter_requests_total", "service", serviceKey)

	if err != nil {
		rl.metrics.IncrementCounter(ctx, "app_rate_limiter_errors_total", "service", serviceKey, "type", "store_error")
	}

	if !allowed {
		rl.metrics.IncrementCounter(ctx, "app_rate_limiter_denied_total", "service", serviceKey)
	}
}

// HTTP Method Implementations - All methods follow the same pattern.

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
