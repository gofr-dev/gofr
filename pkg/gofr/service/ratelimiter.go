package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

// ErrRateLimitExceeded is returned when a request is denied due to rate limiting.
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// RateLimiterConfig holds the configuration for the rate limiter.
type RateLimiterConfig struct {
	RequestsPerSecond float64 // RequestsPerSecond specifies the average number of requests allowed per second.
	Burst             int     // Burst specifies the maximum burst size allowed.
}

// rateLimiter is an HTTP service wrapper that applies rate limiting.
type rateLimiter struct {
	HTTP       // The underlying HTTP service to be rate-limited
	limiter    *rate.Limiter
	Logger            // Gofr's Logger interface
	Metrics           // Gofr's Metrics interface
	serviceURL string // To identify which service is being rate-limited in metrics/logs
}

// newRateLimiter creates a new rateLimiter instance.
func newRateLimiter(svc HTTP, config RateLimiterConfig, logger Logger, metrics Metrics, serviceURL string) HTTP {
	if config.RequestsPerSecond <= 0 || config.Burst <= 0 {
		// Use the existing Logger's Log method with a formatted string.
		logger.Log(fmt.Sprintf("RateLimiterConfig has invalid values (RPS: %f, Burst: %d). Rate limiting will be disabled for service: %s",
			config.RequestsPerSecond, config.Burst, serviceURL))
		return svc // Return the original service if config is invalid
	}

	return &rateLimiter{
		HTTP:       svc,
		limiter:    rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.Burst),
		Logger:     logger,
		Metrics:    metrics,
		serviceURL: serviceURL,
	}
}

func (r *rateLimiter) allowRequest(ctx context.Context, method, path string) error {
	if !r.limiter.Allow() {
		r.Logger.Log(fmt.Sprintf("Rate limit exceeded for service %s, method %s, path %s", r.serviceURL, method, path))

		span := trace.SpanFromContext(ctx)
		if span.IsRecording() {
			span.AddEvent("rate_limit_exceeded", trace.WithAttributes(
				attribute.String("service.url", r.serviceURL),
				attribute.String("http.method", method),
				attribute.String("http.target", path),
			))
		}

		if r.Metrics != nil {
			r.Metrics.RecordHistogram(ctx, "app_http_service_rate_limit_exceeded_total", 1,
				"service_url", r.serviceURL, "method", method)
		}

		return ErrRateLimitExceeded
	}
	return nil
}

func (r *rateLimiter) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodGet, path); err != nil {
		return nil, err
	}
	return r.HTTP.Get(ctx, path, queryParams)
}

func (r *rateLimiter) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodGet, path); err != nil {
		return nil, err
	}
	return r.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (r *rateLimiter) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodPost, path); err != nil {
		return nil, err
	}
	return r.HTTP.Post(ctx, path, queryParams, body)
}

func (r *rateLimiter) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodPost, path); err != nil {
		return nil, err
	}
	return r.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (r *rateLimiter) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodPatch, path); err != nil {
		return nil, err
	}
	return r.HTTP.Patch(ctx, path, queryParams, body)
}

func (r *rateLimiter) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodPatch, path); err != nil {
		return nil, err
	}
	return r.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (r *rateLimiter) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodPut, path); err != nil {
		return nil, err
	}
	return r.HTTP.Put(ctx, path, queryParams, body)
}

func (r *rateLimiter) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodPut, path); err != nil {
		return nil, err
	}
	return r.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (r *rateLimiter) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodDelete, path); err != nil {
		return nil, err
	}
	return r.HTTP.Delete(ctx, path, body)
}

func (r *rateLimiter) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	if err := r.allowRequest(ctx, http.MethodDelete, path); err != nil {
		return nil, err
	}
	return r.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

// HealthCheck delegates to the wrapped HTTP service's HealthCheck method.
func (r *rateLimiter) HealthCheck(ctx context.Context) *Health {
	return r.HTTP.HealthCheck(ctx)
}

// getHealthResponseForEndpoint delegates to the wrapped HTTP service's getHealthResponseForEndpoint method.
func (r *rateLimiter) getHealthResponseForEndpoint(ctx context.Context, endpoint string, timeout int) *Health {
	return r.HTTP.getHealthResponseForEndpoint(ctx, endpoint, timeout)
}
