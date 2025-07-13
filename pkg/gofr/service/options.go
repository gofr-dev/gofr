package service

import (
	"context"
	"net/http"
	"time"
)

type Options interface {
	AddOption(HTTP) HTTP
}

type WithRetries struct {
	MaxRetries int
}

func (w *WithRetries) AddOption(svc HTTP) HTTP {
	return &retryProvider{
		maxRetries: w.MaxRetries,
		HTTP:       svc,
	}
}

type WithOAuth struct {
	AuthFunc func(context.Context, map[string]string) (map[string]string, error)
}

func (w *WithOAuth) AddOption(svc HTTP) HTTP {
	return &authProvider{w.AuthFunc, svc}
}

type WithCircuitBreaker struct {
	Threshold int
	Interval  time.Duration
}

func (w *WithCircuitBreaker) AddOption(svc HTTP) HTTP {
	return NewCircuitBreaker(CircuitBreakerConfig{
		Threshold: w.Threshold,
		Interval:  w.Interval,
	}, svc)
}

type WithRateLimiter struct {
	Config RateLimiterConfig
	Logger
	Metrics
	ServiceURL string
}

func (w *WithRateLimiter) AddOption(svc HTTP) HTTP {
	return newRateLimiter(svc, w.Config, w.Logger, w.Metrics, w.ServiceURL)
}

type WithCustomClient struct {
	Client *http.Client
}

func (w *WithCustomClient) AddOption(svc HTTP) HTTP {
	if hs, ok := svc.(*httpService); ok {
		hs.Client = w.Client
	}
	return svc
}
