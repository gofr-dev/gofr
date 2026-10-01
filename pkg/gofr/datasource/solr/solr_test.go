package solr

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

func Test_InvalidRequest(t *testing.T) {
	client := New(Config{})

	_, _, err := client.call(context.Background(), "GET", ":/localhost:", nil, nil)

	require.Error(t, err, "TEST Failed.\n")
}

func Test_InvalidJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`Not a JSON`))
	}))
	defer ts.Close()

	client := New(Config{})

	_, _, err := client.call(context.Background(), "GET", ts.URL, nil, nil)

	require.Error(t, err, "TEST Failed.\n")
}

func Test_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "some error", http.StatusLocked)
	}))
	ts.Close()

	client := New(Config{})

	_, _, err := client.call(context.Background(), "GET", ts.URL, nil, nil)

	require.Error(t, err, "TEST Failed.\n")
}

func setupClient(t *testing.T) *Client {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
  		"responseHeader": {
    	"rf": 1,
    	"status": 0}}`))
	}))

	t.Cleanup(func() {
		ts.Close()
	})

	a := ts.Listener.Addr().String()
	addr := strings.Split(a, ":")

	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_solr_stats", gomock.Any(), "type", gomock.Any())

	s := New(Config{Host: addr[0], Port: addr[1]})
	s.metrics = mockMetrics
	s.logger = mockLogger

	return s
}

func Test_ClientSearch(t *testing.T) {
	s := setupClient(t)

	resp, err := s.Search(context.Background(), "test", map[string]any{"id": []string{"1234"}})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientCreate(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{
		"id": "1234567",
		"cat": [
			"Book"
		],
		"genere_s": "Hello There"}`)

	resp, err := s.Create(context.Background(), "test", body, map[string]any{"commit": "true"})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientUpdate(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{
		"id": "1234567",
		"cat": [
			"Book"
		]}`)
	resp, err := s.Update(context.Background(), "test", body, map[string]any{"commit": "true"})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientDelete(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{"delete":[
		"1234",
		"12345"
	]}`)

	resp, err := s.Delete(context.Background(), "test", body, map[string]any{"commit": "true"})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientRetrieve(t *testing.T) {
	s := setupClient(t)

	resp, err := s.Retrieve(context.Background(), "test", map[string]any{"wt": "xml"})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientListFields(t *testing.T) {
	s := setupClient(t)

	resp, err := s.ListFields(context.Background(), "test", map[string]any{"includeDynamic": true})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientAddField(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{"add-field":{
		"name":"merchant",
		"type":"string",
		"stored":true }}`)
	resp, err := s.AddField(context.Background(), "test", body)

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientUpdateField(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{"replace-field":{
		"name":"merchant",
		"type":"text_general"}}`)

	resp, err := s.UpdateField(context.Background(), "test", body)

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientDeleteField(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{"delete-field":{
		"name":"merchant",
		"type":"text_general"}}`)

	resp, err := s.DeleteField(context.Background(), "test", body)

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_UseDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	tracer := noop.NewTracerProvider().Tracer("solr")

	tests := []struct {
		desc       string
		logger     any
		metrics    any
		tracer     any
		expLogger  Logger
		expMetrics Metrics
		expTracer  trace.Tracer
	}{
		{
			desc:       "valid dependencies are set",
			logger:     mockLogger,
			metrics:    mockMetrics,
			tracer:     tracer,
			expLogger:  mockLogger,
			expMetrics: mockMetrics,
			expTracer:  tracer,
		},
		{
			desc:    "invalid dependencies are ignored",
			logger:  "invalid",
			metrics: "invalid",
			tracer:  "invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client := New(Config{Host: "localhost", Port: "8983"})

			client.UseLogger(tc.logger)
			client.UseMetrics(tc.metrics)
			client.UseTracer(tc.tracer)

			assert.Equal(t, tc.expLogger, client.logger)
			assert.Equal(t, tc.expMetrics, client.metrics)
			assert.Equal(t, tc.expTracer, client.tracer)
			assert.Equal(t, "http://localhost:8983/solr", client.url)
		})
	}
}

func Test_Connect(t *testing.T) {
	tests := []struct {
		desc      string
		body      string
		expInfo   int
		expErrors int
	}{
		{desc: "successful connection", body: `{"responseHeader":{"status":0}}`, expInfo: 1, expErrors: 0},
		{desc: "health check failure", body: `not a json`, expInfo: 0, expErrors: 1},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/solr/admin/info/system", r.URL.Path)

				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()

			addr := strings.Split(ts.Listener.Addr().String(), ":")

			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			mockLogger.EXPECT().Debugf("connecting to Solr at %v", gomock.Any())
			mockLogger.EXPECT().Debug(gomock.Any())
			mockLogger.EXPECT().Infof("connected to Solr at %v", gomock.Any()).Times(tc.expInfo)
			mockLogger.EXPECT().Errorf("error while connecting to Solr: %v", gomock.Any()).Times(tc.expErrors)
			mockMetrics.EXPECT().NewHistogram("app_solr_stats", gomock.Any(), gomock.Any())
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_solr_stats", gomock.Any(), "type", "HealthCheck")

			client := New(Config{Host: addr[0], Port: addr[1]})
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)

			client.Connect()
		})
	}
}

func Test_CallWithTracer(t *testing.T) {
	tests := []struct {
		desc    string
		method  string
		params  map[string]any
		expCode int
		expData any
	}{
		{
			desc:    "get request with params",
			method:  http.MethodGet,
			params:  map[string]any{"q": "*:*", "fq": []string{"a", "b"}},
			expCode: http.StatusOK,
			expData: map[string]any{"status": "ok"},
		},
		{
			desc:    "post request",
			method:  http.MethodPost,
			params:  nil,
			expCode: http.StatusOK,
			expData: map[string]any{"status": "ok"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))
			defer ts.Close()

			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			mockLogger.EXPECT().Debug(gomock.Any())
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_solr_stats", gomock.Any(), "type", "Search")

			client := New(Config{})
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)
			client.UseTracer(noop.NewTracerProvider().Tracer("solr"))

			resp, span, err := client.call(t.Context(), tc.method, ts.URL, tc.params, nil)
			client.sendOperationStats(t.Context(), &QueryLog{Type: "Search", URL: ts.URL}, time.Now(), "search", span)

			require.NoError(t, err)
			assert.NotNil(t, span)
			assert.Equal(t, Response{Code: tc.expCode, Data: tc.expData}, resp)
		})
	}
}

func Test_HealthCheck(t *testing.T) {
	s := setupClient(t)

	resp, err := s.HealthCheck(t.Context())

	require.NoError(t, err)
	assert.Equal(t, Response{Code: http.StatusOK, Data: map[string]any{
		"responseHeader": map[string]any{"rf": float64(1), "status": float64(0)},
	}}, resp)
}

func Test_QueryLogPrettyPrint(t *testing.T) {
	tests := []struct {
		desc        string
		log         QueryLog
		expContains []string
	}{
		{
			desc:        "whitespace is cleaned",
			log:         QueryLog{Type: "  Search  ", URL: "http://localhost:8983/solr/test/select", Duration: 42},
			expContains: []string{"SOLR", "42", "http://localhost:8983/solr/test/select", "Search\n"},
		},
		{
			desc:        "multiline values are collapsed",
			log:         QueryLog{Type: "Create\n\tDoc", URL: "url", Duration: 1},
			expContains: []string{"SOLR", "Create Doc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer

			tc.log.PrettyPrint(&buf)

			for _, exp := range tc.expContains {
				assert.Contains(t, buf.String(), exp)
			}
		})
	}
}
