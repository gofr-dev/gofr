package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/go-querystring/query"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	APIKey  string
	Model   string
	BaseURL string
}

type Client struct {
	config  *Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

func NewCLient(config *Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com"
	}

	return &Client{
		config: config,
	}
}

func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics interface{}) {
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
		"Duration of OpenAPI requests in seconds",
		openaiHistogramBuckets...,
	)

	c.metrics.NewCounter(
		"openai_api_total_request_count",
		"counts total number of requests made.",
	)

	c.metrics.NewCounterVec(
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
		return response, err
	}

	resp, err := c.Call(ctx, http.MethodPost, url, bytes.NewReader(reqJSON))
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	response, err = io.ReadAll(resp.Body)

	return response, err
}

// Get makes a get request.
func (c *Client) Get(ctx context.Context, url string, input any) (response []byte, err error) {
	if input != nil {
		vals, _ := query.Values(input)
		queryString := vals.Encode()

		if queryString != "" {
			url += "?" + queryString
		}
	}

	resp, err := c.Call(ctx, http.MethodGet, url, nil)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	response, err = io.ReadAll(resp.Body)

	return response, err
}

// Call makes a request.
func (c *Client) Call(ctx context.Context, method, endpoint string, body io.Reader) (response *http.Response, err error) {
	url := c.config.BaseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return response, err
	}

	req.Header.Add("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	return resp, err
}
