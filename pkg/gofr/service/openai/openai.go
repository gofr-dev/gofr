package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	APIKey       string
	Model        string
	BaseURL      string
	Timeout      time.Duration
	MaxIdleConns int
}

type Client struct {
	config     *Config
	logger     Logger
	metrics    Metrics
	tracer     trace.Tracer
	httpClient *http.Client
}

var (
	errorMissingAPIKey = errors.New("api key not provided")
)

type ClientOption func(*Client)

func WithClientHTTP(httpClient *http.Client) func(*Client) {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithClientTimeout(d time.Duration) func(*Client) {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

func NewClient(config *Config, opts ...ClientOption) (*Client, error) {
	if config.APIKey == "" {
		return nil, errorMissingAPIKey
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com"
	}

	if config.Model == "" {
		config.Model = "gpt-4o"
	}

	// Use the provided HTTP client or create a new one with defaults
	c := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:    config.MaxIdleConns,
				IdleConnTimeout: 120 * time.Second,
			},
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

func (c *Client) InitMetrics() {
	openaiHistogramBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}

	c.metrics.NewHistogram(
		"openai_api_request_duration",
		"duration of OpenAPI requests in seconds",
		openaiHistogramBuckets...,
	)

	c.metrics.NewCounter(
		"openai_api_total_request_count",
		"counts total number of requests made.",
	)

	c.metrics.NewUpDownCounter(
		"openai_api_token_usage",
		"counts number of tokens used.",
	)
}

func (c *Client) AddTrace(ctx context.Context, method string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("openai-%v", method))

		return contextWithTrace, span
	}

	return ctx, nil
}

func (c *Client) Post(ctx context.Context, url string, input any) (response []byte, err error) {
	response = make([]byte, 0)

	reqJSON, err := json.Marshal(input)
	if err != nil {
		c.logger.Errorf("%v", err)
		return response, err
	}

	resp, err := c.Call(ctx, http.MethodPost, url, bytes.NewReader(reqJSON))
	if err != nil {
		c.logger.Errorf("%v", err)
		return response, err
	}
	defer resp.Body.Close()

	response, err = io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Errorf("%v", err)
	}

	return response, err
}

// Get makes a get request.
func (c *Client) Get(ctx context.Context, url string) (response []byte, err error) {
	resp, err := c.Call(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.logger.Errorf("%v", err)
		return response, err
	}
	defer resp.Body.Close()

	response, err = io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Errorf("%v", err)
	}

	return response, err
}

// Call makes a request.
func (c *Client) Call(ctx context.Context, method, endpoint string, body io.Reader) (response *http.Response, err error) {
	url := c.config.BaseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		c.logger.Errorf("%v", err)
		return response, err
	}

	req.Header.Add("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Errorf("%v", err)
	}

	return resp, err
}
