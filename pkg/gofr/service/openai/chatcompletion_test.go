package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type test struct {
	name          string
	request       *CreateCompletionsRequest
	response      *CreateCompletionsResponse
	expectedError error
	setupMocks    func(*MockLogger, *MockMetrics)
}

func Test_ChatCompletions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	tests := []test{
		{
			name: "successful completion request",
			request: &CreateCompletionsRequest{
				Messages: []Message{{Role: "user", Content: "Hello"}},
				Model:    "gpt-3.5-turbo",
			},
			response: &CreateCompletionsResponse{
				ID:      "test-id",
				Object:  "chat.completion",
				Created: 1234567890,
				Usage: Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			expectedError: nil,
			setupMocks: func(logger *MockLogger, metrics *MockMetrics) {
				metrics.EXPECT().RecordHistogram(gomock.Any(), "openai_api_request_duration", gomock.Any())
				metrics.EXPECT().RecordRequestCount(gomock.Any(), "openai_api_total_request_count")
				metrics.EXPECT().RecordTokenUsage(gomock.Any(), "openai_api_token_usage", 10, 20)
				logger.EXPECT().Debug(gomock.Any())
			},
		},
		{
			name:          "missing both messages and model",
			request:       &CreateCompletionsRequest{},
			expectedError: errMissingBoth,
			setupMocks: func(logger *MockLogger, _ *MockMetrics) {
				logger.EXPECT().Errorf("%v", errMissingBoth)
			},
		},
		{
			name: "missing messages",
			request: &CreateCompletionsRequest{
				Model: "gpt-3.5-turbo",
			},
			expectedError: errMissingMessages,
			setupMocks: func(logger *MockLogger, _ *MockMetrics) {
				logger.EXPECT().Errorf("%v", errMissingMessages)
			},
		},
		{
			name: "missing model",
			request: &CreateCompletionsRequest{
				Messages: []Message{{Role: "user", Content: "Hello"}},
			},
			expectedError: errMissingModel,
			setupMocks: func(logger *MockLogger, _ *MockMetrics) {
				logger.EXPECT().Errorf("%v", errMissingModel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string

			var server *httptest.Server

			if tt.response != nil {
				server = setupTestServer(t, CompletionsEndpoint, tt.response)
				defer server.Close()
				serverURL = server.URL
			}

			client := &Client{
				config: &Config{
					APIKey:  "test-api-key",
					BaseURL: serverURL,
				},
				httpClient: http.DefaultClient,
				logger:     mockLogger,
				metrics:    mockMetrics,
			}

			tt.setupMocks(mockLogger, mockMetrics)
			response, err := client.CreateCompletions(context.Background(), tt.request)

			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
				assert.Nil(t, response)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, response)
			}
		})
	}
}

func setupTestServer(t *testing.T, path string, response interface{}) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, path, r.URL.Path)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				w.Header().Set("Content-Type", "application/json")
				err := json.NewEncoder(w).Encode(response)

				if err != nil {
					t.Error(err)
					return
				}
			}))

	return server
}
