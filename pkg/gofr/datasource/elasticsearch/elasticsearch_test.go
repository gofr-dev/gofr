package elasticsearch

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

var (
	errTestFailed = errors.New("test operation failed")
)

// mockTransport implements the elasticsearch.Transport interface for testing.
type mockTransport struct {
	response *http.Response
	err      error
}

func (t *mockTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return t.response, t.err
}

func (t *mockTransport) Perform(*http.Request) (*http.Response, error) {
	return t.response, t.err
}

func createMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header: http.Header{"Content-Type": []string{"application/json"},
			"X-Elastic-Product": []string{"Elasticsearch"}},
	}
}

// setupTest creates a client with mocked components for testing.
func setupTest(t *testing.T) (*Client, *mockTransport) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockTransport := &mockTransport{}

	// Create a client with a custom transport
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Transport:               mockTransport,
		DiscoverNodesOnStart:    false,
		EnableMetrics:           false,
		EnableCompatibilityMode: true,
	})
	require.NoError(t, err)

	config := Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  "elastic",
		Password:  "changeme",
	}

	client := New(config)
	client.client = es // Replace the client with our mocked version
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-elasticsearch"))

	// Setup common expectations for metrics and logging
	mockMetrics.EXPECT().NewHistogram("es_request_duration_ms", gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "es_request_duration_ms", gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()

	return client, mockTransport
}

func TestClient_CreateIndex_Success(t *testing.T) {
	client, transport := setupTest(t)

	transport.response = createMockResponse(200, `{"acknowledged": true}`)
	defer transport.response.Body.Close()

	transport.err = nil

	settings := map[string]any{
		"settings": map[string]any{
			"number_of_shards": 1,
		},
	}

	err := client.CreateIndex(t.Context(), "test-index", settings)
	require.NoError(t, err)
}

func TestClient_CreateIndex_Errors(t *testing.T) {
	resp := createMockResponse(400, `{"error": "index already exists"}`)
	defer resp.Body.Close()

	tests := []struct {
		name       string
		index      string
		settings   map[string]any
		resp       *http.Response
		httpErr    error
		errMessage string
	}{
		{
			name:       "empty index name",
			index:      "",
			settings:   map[string]any{},
			errMessage: "index name cannot be empty",
		},
		{
			name:       "elasticsearch operation error",
			index:      "test-index",
			settings:   map[string]any{},
			httpErr:    errTestFailed,
			errMessage: "elasticsearch operation error",
		},
		{
			name:       "elasticsearch response error",
			index:      "test-index",
			settings:   map[string]any{},
			resp:       resp,
			errMessage: "invalid elasticsearch response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTest(t)
			transport.response = tt.resp
			transport.err = tt.httpErr

			err := client.CreateIndex(t.Context(), tt.index, tt.settings)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errMessage)
		})
	}
}

func TestClient_DeleteIndex_Success(t *testing.T) {
	client, transport := setupTest(t)

	transport.response = createMockResponse(200, `{"acknowledged": true}`)
	transport.err = nil

	defer transport.response.Body.Close()

	err := client.DeleteIndex(t.Context(), "test-index")
	require.NoError(t, err)
}

func TestClient_DeleteIndex_Errors(t *testing.T) {
	resp := createMockResponse(404, `{"error": "index not found"}`)
	defer resp.Body.Close()

	tests := []struct {
		name       string
		index      string
		resp       *http.Response
		httpErr    error
		errMessage string
	}{
		{
			name:       "empty index name",
			index:      "",
			errMessage: "index name cannot be empty",
		},
		{
			name:       "elasticsearch operation error",
			index:      "test-index",
			httpErr:    errTestFailed,
			errMessage: "elasticsearch operation error",
		},
		{
			name:       "elasticsearch response error",
			index:      "test-index",
			resp:       resp,
			errMessage: "invalid elasticsearch response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTest(t)
			transport.response = tt.resp
			transport.err = tt.httpErr

			err := client.DeleteIndex(t.Context(), tt.index)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errMessage)
		})
	}
}

func TestClient_IndexDocument_Success(t *testing.T) {
	client, transport := setupTest(t)

	transport.response = createMockResponse(201, `{"result":"created"}`)
	transport.err = nil

	defer transport.response.Body.Close()

	document := map[string]any{
		"title": "Test Document",
	}

	err := client.IndexDocument(t.Context(), "test-index", "123", document)
	require.NoError(t, err)
}

func TestClient_IndexDocument_Errors(t *testing.T) {
	resp := createMockResponse(400, `{"error":"bad request"}`)
	defer resp.Body.Close()

	tests := []struct {
		name       string
		index      string
		id         string
		document   any
		resp       *http.Response
		httpErr    error
		errMessage string
	}{
		{
			name:       "empty index",
			index:      "",
			id:         "123",
			document:   map[string]any{},
			errMessage: "index name cannot be empty",
		},
		{
			name:       "empty document ID",
			index:      "test-index",
			id:         "",
			document:   map[string]any{},
			errMessage: "document ID cannot be empty",
		},
		{
			name:       "json `marshaling` error",
			index:      "test-index",
			id:         "123",
			document:   make(chan int),
			errMessage: "error marshaling data",
		},
		{
			name:       "elasticsearch operation error",
			index:      "test-index",
			id:         "123",
			document:   map[string]any{},
			httpErr:    errTestFailed,
			errMessage: "elasticsearch operation error",
		},
		{
			name:       "elasticsearch response error",
			index:      "test-index",
			id:         "123",
			document:   map[string]any{},
			resp:       resp,
			errMessage: "invalid elasticsearch response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTest(t)
			transport.response = tt.resp
			transport.err = tt.httpErr

			err := client.IndexDocument(t.Context(), tt.index, tt.id, tt.document)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errMessage)
		})
	}
}

func TestClient_GetDocument_Success(t *testing.T) {
	client, transport := setupTest(t)

	responseBody := `{"_id":"123", "_source": {"title": "Test Document"}}`
	transport.response = createMockResponse(200, responseBody)
	transport.err = nil

	defer transport.response.Body.Close()

	result, err := client.GetDocument(t.Context(), "test-index", "123")
	require.NoError(t, err)
	require.Equal(t, "123", result["_id"])
	require.Equal(t, map[string]any{"title": "Test Document"}, result["_source"])
}

func TestClient_GetDocument_Errors(t *testing.T) {
	invalidJSONResp := createMockResponse(200, `{"_source":`)
	defer invalidJSONResp.Body.Close()

	notFoundResp := createMockResponse(404, `{"error":"not found"}`)
	defer notFoundResp.Body.Close()

	tests := []struct {
		name       string
		index      string
		id         string
		resp       *http.Response
		httpErr    error
		errMessage string
	}{
		{
			name:       "empty index",
			index:      "",
			id:         "123",
			errMessage: "index name cannot be empty",
		},
		{
			name:       "empty document ID",
			index:      "test-index",
			id:         "",
			errMessage: "document ID cannot be empty",
		},
		{
			name:       "elasticsearch operation error",
			index:      "test-index",
			id:         "123",
			httpErr:    errTestFailed,
			errMessage: "elasticsearch operation error",
		},
		{
			name:       "elasticsearch response error",
			index:      "test-index",
			id:         "123",
			resp:       createMockResponse(404, `{"error":"not found"}`),
			errMessage: "invalid elasticsearch response",
		},
		{
			name:       "json decoding error",
			index:      "test-index",
			id:         "123",
			resp:       invalidJSONResp,
			errMessage: "error parsing response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTest(t)
			transport.response = tt.resp
			transport.err = tt.httpErr

			_, err := client.GetDocument(t.Context(), tt.index, tt.id)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errMessage)
		})
	}
}

func TestClient_UpdateDocument_Success(t *testing.T) {
	client, transport := setupTest(t)

	transport.response = createMockResponse(200, `{"result":"updated"}`)
	transport.err = nil

	defer transport.response.Body.Close()

	err := client.UpdateDocument(t.Context(), "test-index", "123", map[string]any{
		"name": "updated name",
	})
	require.NoError(t, err)
}

func TestClient_UpdateDocument_Errors(t *testing.T) {
	errResp := createMockResponse(500, `{"error":"internal error"}`)
	defer errResp.Body.Close()

	tests := []struct {
		name        string
		index       string
		id          string
		update      map[string]any
		response    *http.Response
		err         error
		expectedMsg string
	}{
		{
			name:        "empty index",
			index:       "",
			id:          "123",
			update:      map[string]any{"field": "value"},
			expectedMsg: "index name cannot be empty",
		},
		{
			name:        "empty document ID",
			index:       "test-index",
			id:          "",
			update:      map[string]any{"field": "value"},
			expectedMsg: "document ID cannot be empty",
		},
		{
			name:        "empty update map",
			index:       "test-index",
			id:          "123",
			update:      map[string]any{},
			expectedMsg: "query cannot be empty",
		},
		{
			name:        "json marshal error",
			index:       "test-index",
			id:          "123",
			update:      map[string]any{"field": make(chan int)},
			expectedMsg: "error marshaling data: update: json: unsupported type: chan int",
		},
		{
			name:        "elasticsearch transport error",
			index:       "test-index",
			id:          "123",
			update:      map[string]any{"field": "value"},
			err:         errOperation,
			expectedMsg: "elasticsearch operation error: updating document",
		},
		{
			name:        "elasticsearch error response",
			index:       "test-index",
			id:          "123",
			update:      map[string]any{"field": "value"},
			response:    errResp,
			expectedMsg: "invalid elasticsearch response: [500 Internal Server Error]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTest(t)

			transport.response = tt.response
			transport.err = tt.err

			err := client.UpdateDocument(t.Context(), tt.index, tt.id, tt.update)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedMsg)
		})
	}
}

func TestClient_DeleteDocument_Success(t *testing.T) {
	client, transport := setupTest(t)

	transport.response = createMockResponse(200, `{"result":"deleted"}`)
	transport.err = nil

	defer transport.response.Body.Close()

	err := client.DeleteDocument(t.Context(), "test-index", "123")
	require.NoError(t, err)
}

func TestClient_DeleteDocument_Errors(t *testing.T) {
	notFoundResp := createMockResponse(500, `{"error":"not found"}`)
	defer notFoundResp.Body.Close()

	tests := []struct {
		name        string
		index       string
		id          string
		response    *http.Response
		err         error
		expectedMsg string
	}{
		{
			name:        "empty index",
			index:       "",
			id:          "123",
			expectedMsg: "index name cannot be empty",
		},
		{
			name:        "empty document ID",
			index:       "test-index",
			id:          "",
			expectedMsg: "document ID cannot be empty",
		},
		{
			name:        "elasticsearch transport error",
			index:       "test-index",
			id:          "123",
			err:         errOperation,
			expectedMsg: "elasticsearch operation error: deleting document",
		},
		{
			name:        "elasticsearch error response",
			index:       "test-index",
			id:          "123",
			response:    createMockResponse(500, `{"error":"not found"}`),
			expectedMsg: "invalid elasticsearch response: [500 Internal Server Error]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTest(t)

			transport.response = tt.response
			transport.err = tt.err

			err := client.DeleteDocument(t.Context(), tt.index, tt.id)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedMsg)
		})
	}
}

func TestClient_Search_Success(t *testing.T) {
	client, transport := setupTest(t)

	responseBody := `{
		"took": 15,
		"hits": {
			"total": {"value": 1},
			"hits": [{"_id": "1", "_source": {"title": "Test"}}]
		}
	}`
	transport.response = createMockResponse(200, responseBody)
	transport.err = nil

	defer transport.response.Body.Close()

	query := map[string]any{
		"query": map[string]any{
			"match_all": map[string]any{},
		},
	}

	result, err := client.Search(t.Context(), []string{"test-index"}, query)

	require.NoError(t, err)
	require.InDelta(t, 1.0, result["hits"].(map[string]any)["total"].(map[string]any)["value"], 0.0001)
}

func TestClient_Search_Errors(t *testing.T) {
	internalServerErrorResp := createMockResponse(500, `{"error": "internal server error"}`)
	defer internalServerErrorResp.Body.Close()

	invalidJSONResp := createMockResponse(200, `{"invalid": json`)
	defer invalidJSONResp.Body.Close()

	tests := []struct {
		name        string
		indices     []string
		query       map[string]any
		response    *http.Response
		err         error
		expectedMsg string
	}{
		{
			name:        "empty indices",
			indices:     []string{},
			query:       map[string]any{"query": map[string]any{}},
			expectedMsg: "index name cannot be empty",
		},
		{
			name:        "nil query",
			indices:     []string{"test-index"},
			query:       nil,
			expectedMsg: "query cannot be empty",
		},
		{
			name:        "elasticsearch transport error",
			indices:     []string{"test-index"},
			query:       map[string]any{"query": map[string]any{}},
			err:         errOperation,
			expectedMsg: "elasticsearch operation error: executing search",
		},
		{
			name:        "elasticsearch error response",
			indices:     []string{"test-index"},
			query:       map[string]any{"query": map[string]any{}},
			response:    internalServerErrorResp,
			expectedMsg: "invalid elasticsearch response",
		},
		{
			name:        "invalid json response",
			indices:     []string{"test-index"},
			query:       map[string]any{"query": map[string]any{}},
			response:    invalidJSONResp,
			expectedMsg: "error parsing response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTest(t)

			transport.response = tt.response
			transport.err = tt.err

			_, err := client.Search(t.Context(), tt.indices, tt.query)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedMsg)
		})
	}
}

func TestClient_Bulk_Success(t *testing.T) {
	client, transport := setupTest(t)

	responseBody := `{
		"took": 30,
		"errors": false,
		"items": [
			{"index": {"status": 201}},
			{"index": {"status": 201}}
		]
	}`
	transport.response = createMockResponse(200, responseBody)
	transport.err = nil

	defer transport.response.Body.Close()

	operations := []map[string]any{
		{"index": map[string]any{"_index": "test-index", "_id": "1"}},
		{"title": "Document 1"},
		{"index": map[string]any{"_index": "test-index", "_id": "2"}},
		{"title": "Document 2"},
	}

	result, err := client.Bulk(t.Context(), operations)

	require.NoError(t, err)
	require.False(t, result["errors"].(bool))
	require.Len(t, result["items"].([]any), 2)
}

func TestClient_Bulk_Errors(t *testing.T) {
	invalidJSONResp := createMockResponse(200, `{"invalid": json`)
	defer invalidJSONResp.Body.Close()

	badRequestResp := createMockResponse(400, `{"error": "bad request"}`)
	defer badRequestResp.Body.Close()

	tests := []struct {
		name        string
		operations  []map[string]any
		response    *http.Response
		err         error
		expectedMsg string
	}{
		{
			name:        "empty operations",
			operations:  []map[string]any{},
			expectedMsg: "operations cannot be empty",
		},
		{
			name:        "json marshal error",
			operations:  []map[string]any{{"invalid": make(chan int)}},
			expectedMsg: "error encoding operation",
		},
		{
			name:        "elasticsearch transport error",
			operations:  []map[string]any{{"index": map[string]any{"_index": "test-index"}}},
			err:         errOperation,
			expectedMsg: "elasticsearch operation error: executing bulk",
		},
		{
			name:        "elasticsearch error response",
			operations:  []map[string]any{{"index": map[string]any{"_index": "test-index"}}},
			response:    badRequestResp,
			expectedMsg: "invalid elasticsearch response",
		},
		{
			name:        "invalid json response",
			operations:  []map[string]any{{"index": map[string]any{"_index": "test-index"}}},
			response:    invalidJSONResp,
			expectedMsg: "error parsing response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTest(t)

			transport.response = tt.response
			transport.err = tt.err

			_, err := client.Bulk(t.Context(), tt.operations)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedMsg)
		})
	}
}

func TestClient_Connect_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	client, _ := setupTest(t)
	mockLogger := NewMockLogger(ctrl)

	client.UseLogger(mockLogger)

	mux := http.NewServeMux()

	// Handle ping endpoint
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)

		_, err := w.Write([]byte(`{}`))
		if err != nil {
			t.Error("failed to write response: ", err)
		}
	})

	// Handle info endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)

		_, err := w.Write([]byte(`{
   "name": "test-node",
   "cluster_name": "test-cluster",
   "version": {
    "number": "8.0.0"
   }
  }`))
		if err != nil {
			t.Error("failed to write response: ", err)
		}
	})

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf("Elasticsearch health check failed: %v", gomock.Any())

	server := httptest.NewServer(mux)
	defer server.Close()

	client.Connect()

	require.NotNil(t, client.client, "Elasticsearch client should be initialized")
}
