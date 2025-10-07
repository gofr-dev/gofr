package gofr

import (
	"net/http"
	"sync"
	"time"
)

type HTTPClientOptions struct {
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
}

//nolint:gochecknoglobals // HTTP client
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
	defaultOptions = HTTPClientOptions{
		Timeout:             30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,
	}
)

func GetHTTPClient(opts ...HTTPClientOptions) *http.Client {
	if len(opts) > 0 {
		options := opts[0]

		return &http.Client{
			Timeout: options.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        options.MaxIdleConns,
				MaxIdleConnsPerHost: options.MaxIdleConnsPerHost,
				MaxConnsPerHost:     options.MaxConnsPerHost,
				IdleConnTimeout:     options.IdleConnTimeout,
			},
		}
	}

	httpClientOnce.Do(func() {
		httpClient = &http.Client{
			Timeout: defaultOptions.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        defaultOptions.MaxIdleConns,
				MaxIdleConnsPerHost: defaultOptions.MaxIdleConnsPerHost,
				MaxConnsPerHost:     defaultOptions.MaxConnsPerHost,
				IdleConnTimeout:     defaultOptions.IdleConnTimeout,
			},
		}
	})

	return httpClient
}
