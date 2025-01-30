package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"
)

var (
	errMockRead     = errors.New("read error")
	errNetworkError = errors.New("network error")
)

func Test_NewClient(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		opts          []ClientOption
		baseURL       string
		expected      string
		timeout       time.Duration
		expectedError error
	}{
		{
			name:          "with default base URL",
			config:        &Config{APIKey: "test-key", Model: "gpt-4"},
			opts:          []ClientOption{WithClientHTTP(&http.Client{})},
			expected:      "https://api.openai.com",
			expectedError: nil,
		},
		{
			name:          "with custom base URL",
			config:        &Config{APIKey: "test-key", Model: "gpt-4", BaseURL: "https://custom.openai.com"},
			opts:          []ClientOption{WithClientHTTP(&http.Client{})},
			expected:      "https://custom.openai.com",
			expectedError: nil,
		},
		{
			name:          "missing api key",
			config:        &Config{Model: "gpt-4"},
			opts:          []ClientOption{WithClientHTTP(&http.Client{})},
			expectedError: errorMissingAPIKey,
		},
		{
			name:     "with custom timeout",
			config:   &Config{APIKey: "test-key", Model: "gpt-4"},
			opts:     []ClientOption{WithClientTimeout(5 * time.Second)},
			expected: "https://api.openai.com",
			timeout:  5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, tt.opts...)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				assert.Nil(t, client)
			} else {
				assert.Equal(t, tt.expected, client.config.BaseURL)

				if tt.timeout > 0 {
					assert.Equal(t, tt.timeout, client.httpClient.Timeout)
				}

				require.NoError(t, err)
			}
		})
	}
}

func Test_UseLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)

	config := &Config{
		APIKey: "key",
	}

	client, _ := NewClient(config)
	client.UseLogger(mockLogger)

	assert.NotNil(t, client.logger)
}

func Test_UseMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		APIKey: "key",
	}

	client, _ := NewClient(config)
	client.UseMetrics(mockMetrics)

	assert.NotNil(t, client.metrics)
}

func Test_UseTracer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tracer := trace.NewNoopTracerProvider().Tracer("test-tracer")

	config := &Config{
		APIKey: "key",
	}

	client, _ := NewClient(config)
	client.UseTracer(tracer)

	assert.NotNil(t, client.tracer)
}

func Test_InitMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		APIKey: "key",
	}

	client, _ := NewClient(config)
	client.UseMetrics(mockMetrics)

	openaiHistogramBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}

	mockMetrics.EXPECT().NewHistogram(
		"openai_api_request_duration",
		"duration of OpenAPI requests in seconds",
		openaiHistogramBuckets,
	)

	mockMetrics.EXPECT().NewCounter(
		"openai_api_total_request_count",
		"counts total number of requests made.",
	)

	mockMetrics.EXPECT().NewUpDownCounter(
		"openai_api_token_usage",
		"counts number of tokens used.",
	)

	client.InitMetrics()
}

func Test_AddTrace(t *testing.T) {
	config := &Config{
		APIKey: "test-key",
	}

	client, _ := NewClient(config)
	tracer := trace.NewNoopTracerProvider().Tracer("test-tracer")
	client.UseTracer(tracer)

	ctx := context.Background()
	resultCtx, span := client.AddTrace(ctx, "test-method")

	assert.NotNil(t, span)
	assert.NotEqual(t, ctx, resultCtx)
}

type mockTransport struct {
	response *http.Response
	err      error
}

func (m *mockTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return m.response, m.err
}

func Test_Call(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com",
	}

	tests := []struct {
		name       string
		method     string
		endpoint   string
		body       io.Reader
		setupMocks func(*http.Client)
		wantErr    bool
	}{
		{
			name:     "successful request",
			method:   http.MethodPost,
			endpoint: "/v1/chat/completions",
			body:     strings.NewReader(`{"test":"data"}`),
			setupMocks: func(client *http.Client) {
				client.Transport = &mockTransport{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"response":"success"}`)),
					},
				}
			},
			wantErr: false,
		},
		{
			name:     "failed request",
			method:   http.MethodPost,
			endpoint: "/v1/chat/completions",
			body:     strings.NewReader(`{"test":"data"}`),
			setupMocks: func(client *http.Client) {
				client.Transport = &mockTransport{
					err: errNetworkError,
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient := &http.Client{}
			if tt.setupMocks != nil {
				tt.setupMocks(httpClient)
			}

			client, _ := NewClient(config, WithClientHTTP(httpClient))
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)

			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

			resp, err := client.Call(context.Background(), tt.method, tt.endpoint, tt.body)
			if resp != nil {
				defer resp.Body.Close()
			}

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			}
		})
	}
}
func Test_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com",
	}

	tests := []struct {
		name       string
		url        string
		setupMocks func(*http.Client)
		want       []byte
		wantErr    bool
	}{
		{
			name: "successful GET request",
			url:  "/v1/models",
			setupMocks: func(client *http.Client) {
				client.Transport = &mockTransport{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"data":"test"}`)),
					},
				}
			},
			want:    []byte(`{"data":"test"}`),
			wantErr: false,
		},
		{
			name: "network error",
			url:  "/v1/models",
			setupMocks: func(client *http.Client) {
				client.Transport = &mockTransport{
					err: errNetworkError,
				}
			},
			want:    []byte{},
			wantErr: true,
		},
		{
			name: "error reading response body",
			url:  "/v1/models",
			setupMocks: func(client *http.Client) {
				client.Transport = &mockTransport{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(&errorReader{}),
					},
				}
			},
			want:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient := &http.Client{}
			if tt.setupMocks != nil {
				tt.setupMocks(httpClient)
			}

			client, _ := NewClient(config, WithClientHTTP(httpClient))
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)

			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

			got, err := client.Get(context.Background(), tt.url)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// ErrMockRead is a static error for mock read operations

// errorReader is a mock reader that always returns an error.
type errorReader struct{}

func (*errorReader) Read(_ []byte) (n int, err error) {
	return 0, errMockRead
}

func Test_Post(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com",
	}

	tests := []struct {
		name       string
		url        string
		input      any
		setupMocks func(*http.Client)
		want       []byte
		wantErr    bool
	}{
		{
			name: "successful POST request",
			url:  "/v1/completions",
			input: map[string]string{
				"prompt": "test",
			},
			setupMocks: func(client *http.Client) {
				client.Transport = &mockTransport{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"result":"success"}`)),
					},
				}
			},
			want:    []byte(`{"result":"success"}`),
			wantErr: false,
		},
		{
			name:       "invalid input JSON",
			url:        "/v1/completions",
			input:      make(chan int), // Unmarshalable type
			setupMocks: func(_ *http.Client) {},
			want:       []byte{},
			wantErr:    true,
		},
		{
			name: "network error",
			url:  "/v1/completions",
			input: map[string]string{
				"prompt": "test",
			},
			setupMocks: func(client *http.Client) {
				client.Transport = &mockTransport{
					err: errNetworkError,
				}
			},
			want:    []byte{},
			wantErr: true,
		},
		{
			name: "error reading response body",
			url:  "/v1/completions",
			input: map[string]string{
				"prompt": "test",
			},
			setupMocks: func(client *http.Client) {
				client.Transport = &mockTransport{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(&errorReader{}),
					},
				}
			},
			want:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient := &http.Client{}
			if tt.setupMocks != nil {
				tt.setupMocks(httpClient)
			}

			client, _ := NewClient(config, WithClientHTTP(httpClient))
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)

			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

			got, err := client.Post(context.Background(), tt.url, tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
