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

//nolint:funlen // Function length is intentional due to complexity
func Test_ChatCompletions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	tests := []struct {
		name          string
		request       *CreateCompletionsRequest
		response      *CreateCompletionsResponse
		expectedError error
		setupMocks    func(*MockLogger, *MockMetrics)
	}{
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
				Usage: struct {
					PromptTokens           int         `json:"prompt_tokens,omitempty"`
					CompletionTokens       int         `json:"completion_tokens,omitempty"`
					TotalTokens            int         `json:"total_tokens,omitempty"`
					CompletionTokelDetails interface{} `json:"completion_tokens_details,omitempty"`
					PromptTokenDetails     interface{} `json:"prompt_tokens_details,omitempty"`
				}{
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
			expectedError: ErrMissingBoth,
			setupMocks: func(logger *MockLogger, _ *MockMetrics) {
				logger.EXPECT().Errorf("%v", ErrMissingBoth)
			},
		},
		{
			name: "missing messages",
			request: &CreateCompletionsRequest{
				Model: "gpt-3.5-turbo",
			},
			expectedError: ErrMissingMessages,
			setupMocks: func(logger *MockLogger, _ *MockMetrics) {
				logger.EXPECT().Errorf("%v", ErrMissingMessages)
			},
		},
		{
			name: "missing model",
			request: &CreateCompletionsRequest{
				Messages: []Message{{Role: "user", Content: "Hello"}},
			},
			expectedError: ErrMissingModel,
			setupMocks: func(logger *MockLogger, _ *MockMetrics) {
				logger.EXPECT().Errorf("%v", ErrMissingModel)
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
				logger:  mockLogger,
				metrics: mockMetrics,
			}

			tt.setupMocks(mockLogger, mockMetrics)

			response, err := client.CreateCompletions(context.Background(), tt.request)

			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
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
