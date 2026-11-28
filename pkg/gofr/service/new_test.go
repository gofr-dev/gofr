package service

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"

	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// TestMethodQueryMatchesInboundConstant pins the two copies of the QUERY method
// string together. The outbound client keeps its own constant rather than
// importing gofr/http (see the comment on methodQuery), so nothing but this
// assertion stops the two from drifting -- and a drift would be silent: the
// outbound call would simply use a method no GoFr server routes.
func TestMethodQueryMatchesInboundConstant(t *testing.T) {
	assert.Equal(t, gofrHTTP.MethodQuery, methodQuery,
		"the outbound QUERY method must be the same string the inbound router registers")
}

func TestNewHTTPService(t *testing.T) {
	tests := []struct {
		desc           string
		serviceAddress string
	}{
		{"Valid Address", "http://example.com"},
		{"Empty Address", ""},
		{"Invalid Address", "not_a_valid_address"},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			service := NewHTTPService(tc.serviceAddress, nil, nil)
			assert.NotNil(t, service, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}

	t.Run("WithAttributes", func(t *testing.T) {
		service := NewHTTPService("http://example.com", nil, nil, WithAttributes(map[string]string{"name": "test-service"}))
		httpSvc := service.(*httpService)
		assert.Equal(t, "test-service", httpSvc.name)
	})
}

func TestHTTPService_createAndSendRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	ctx := t.Context()

	tests := []struct {
		desc           string
		queryParams    map[string]any
		body           []byte
		headers        map[string]string
		expQueryParam  string
		expContentType string
	}{
		{"with query params, body and header", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
			[]byte("{Test Body}"), map[string]string{"header1": "value1"}, "key=value&name=gofr&name=test", "application/json"},
		{"with query params, body, header and content type", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
			[]byte("{Test Body}"), map[string]string{"header1": "value1", "content-type": "application/json"},
			"key=value&name=gofr&name=test", "application/json"},
		{"with query params, body, header and content type xml", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
			[]byte("{Test Body}"), map[string]string{"header1": "value1", "content-type": "application/xml"},
			"key=value&name=gofr&name=test", "application/xml"},
		{"without query params, body, header and content type", nil, []byte("{Test Body}"),
			map[string]string{"header1": "value1", "content-type": "application/json"},
			"", "application/json"},
	}

	for i, tc := range tests {
		// Setup a test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// read request body
			var body []byte

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal("Unable to read request body")
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/test-path", r.URL.Path)
			assert.Equal(t, tc.expQueryParam, r.URL.RawQuery)
			assert.Contains(t, "value1", r.Header.Get("Header1"))
			assert.Contains(t, tc.expContentType, r.Header.Get("Content-Type"))
			assert.Equal(t, string(tc.body), string(body))

			w.WriteHeader(http.StatusOK)
		}))

		service := &httpService{
			Client:         http.DefaultClient,
			url:            server.URL,
			Tracer:         trace.NewTracerProvider().Tracer("gofr-http-client"),
			Logger:         logging.NewMockLogger(logging.INFO),
			Metrics:        metrics,
			healthEndpoint: "",
			healthTimeout:  0,
		}

		metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", server.URL,
			"method", http.MethodPost, "status", fmt.Sprintf("%v", http.StatusOK)).Times(1)

		resp, err := service.createAndSendRequest(ctx,
			http.MethodPost, "test-path", tc.queryParams, tc.body, tc.headers)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
		}

		require.NoError(t, err)
		assert.NotNil(t, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		server.Close()
	}
}

func TestHTTPService_Get(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.Get(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_GetWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("Header1"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.GetWithHeaders(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}},
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Put(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.Put(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_PutWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("Header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.PutWithHeaders(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Patch(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.Patch(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_PatchWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("Header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.PatchWithHeaders(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Post(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.Post(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_PostWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("Header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.PostWithHeaders(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Delete(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.Delete(t.Context(), "test-path", []byte("{Test Body}"))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_DeleteWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Contains(t, "value1", r.Header.Get("Header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.DeleteWithHeaders(t.Context(), "test-path", []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_createAndSendRequestCreateRequestFailure(t *testing.T) {
	service := &httpService{
		Client: http.DefaultClient,
		Tracer: trace.NewTracerProvider().Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	ctx := t.Context()
	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		"!@#$", "test-path", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, true)
}

func TestHTTPService_createAndSendRequestServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	service := &httpService{
		Client:  http.DefaultClient,
		Tracer:  trace.NewTracerProvider().Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.INFO),
		Metrics: metrics,
	}

	ctx := t.Context()

	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", gomock.Any(),
		"method", http.MethodPost, "status", fmt.Sprintf("%v", http.StatusServiceUnavailable))

	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		http.MethodPost, "test-path", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, true)
}

func validateResponse(t *testing.T, resp *http.Response, err error, hasError bool) {
	t.Helper()

	if resp != nil {
		defer resp.Body.Close()
	}

	if hasError {
		require.Error(t, err)
		assert.Nil(t, resp, "TEST[%d], Failed.\n%s")

		return
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST[%d], Failed.\n%s")
}

func newService(t *testing.T, server *httptest.Server) *httpService {
	t.Helper()

	tp := trace.NewTracerProvider()

	return &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: tp.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}
}

func TestHTTPService_Query(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		assert.Equal(t, "QUERY", r.Method)
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "index=books", r.URL.RawQuery)
		assert.JSONEq(t, `{"filter":"title"}`, string(body))
		// Content-Type defaults to application/json when the caller sets none.
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.Query(t.Context(), "search",
		map[string]any{"index": "books"}, []byte(`{"filter":"title"}`))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_QueryWithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		assert.Equal(t, "QUERY", r.Method)
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "application/sql", r.Header.Get("Content-Type"))
		assert.Equal(t, "SELECT 1", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.QueryWithHeaders(t.Context(), "search", nil, []byte("SELECT 1"),
		map[string]string{"content-type": "application/sql"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Query_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		assert.Equal(t, "QUERY", r.Method)
		assert.Empty(t, string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := newService(t, server)
	resp, err := service.Query(t.Context(), "search", nil, nil)

	validateResponse(t, resp, err, false)
}

func TestNewHTTPService_CrossHostRedirectDropsAPIKey(t *testing.T) {
	var (
		forwarded atomic.Value // string
		landed    atomic.Bool
	)

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		landed.Store(true)
		forwarded.Store(r.Header.Get(xAPIKeyHeader))

		w.WriteHeader(http.StatusOK)
	}))
	defer other.Close()

	// Keep the port and change only the host name: the redirect must leave
	// 127.0.0.1 for localhost, which is a different host for net/http.
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, otherURL+"/landing", http.StatusFound)
	}))
	defer upstream.Close()

	apiKey, err := NewAPIKeyConfig("secret-key")
	require.NoError(t, err)

	service := NewHTTPService(upstream.URL, logging.NewMockLogger(logging.ERROR), nil, apiKey)

	resp, err := service.Get(t.Context(), "orders", nil)
	require.NoError(t, err)

	defer resp.Body.Close()

	// Without this the test passes even when the redirect is never followed.
	require.True(t, landed.Load(), "the redirect never reached the second server")
	assert.Empty(t, forwarded.Load())
}

func TestNewHTTPService_RedirectBackToOriginKeepsAPIKey(t *testing.T) {
	var (
		atB     atomic.Value // string
		backAtA atomic.Value // string
		landedB atomic.Bool
		landedA atomic.Bool
	)

	var (
		serverA *httptest.Server
		urlA    string
	)

	// B lives on 127.0.0.1, A is reached as localhost: the first hop changes the
	// host, the hop back lands on the original host again.
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		landedB.Store(true)
		atB.Store(r.Header.Get(xAPIKeyHeader))

		http.Redirect(w, r, urlA+"/back", http.StatusFound)
	}))
	defer serverB.Close()

	serverA = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/back" {
			landedA.Store(true)
			backAtA.Store(r.Header.Get(xAPIKeyHeader))

			w.WriteHeader(http.StatusOK)

			return
		}

		http.Redirect(w, r, serverB.URL+"/hop", http.StatusFound)
	}))
	defer serverA.Close()

	urlA = strings.Replace(serverA.URL, "127.0.0.1", "localhost", 1)

	apiKey, err := NewAPIKeyConfig("secret-key")
	require.NoError(t, err)

	service := NewHTTPService(urlA, logging.NewMockLogger(logging.ERROR), nil, apiKey)

	resp, err := service.Get(t.Context(), "orders", nil)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.True(t, landedB.Load(), "the first redirect never reached the second server")
	require.True(t, landedA.Load(), "the redirect back to the origin never landed")

	// A to B drops the key, B back to A restores it because A is the original host.
	assert.Empty(t, atB.Load())
	assert.Equal(t, "secret-key", backAtA.Load())
}

func TestNewHTTPService_CrossHostRedirect307DropsAPIKey(t *testing.T) {
	var (
		forwarded atomic.Value // string
		body      atomic.Value // string
		landed    atomic.Bool
	)

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		landed.Store(true)
		forwarded.Store(r.Header.Get(xAPIKeyHeader))

		b, _ := io.ReadAll(r.Body)
		body.Store(string(b))

		w.WriteHeader(http.StatusOK)
	}))
	defer other.Close()

	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", otherURL+"/landing")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()

	apiKey, err := NewAPIKeyConfig("secret-key")
	require.NoError(t, err)

	service := NewHTTPService(upstream.URL, logging.NewMockLogger(logging.ERROR), nil, apiKey)

	resp, err := service.Post(t.Context(), "orders", nil, []byte("payload"))
	require.NoError(t, err)

	defer resp.Body.Close()

	require.True(t, landed.Load(), "the 307 redirect never reached the second server")
	assert.Empty(t, forwarded.Load())
	assert.Equal(t, "payload", body.Load())
}

func TestNewHTTPService_StopsAfterMaxRedirects(t *testing.T) {
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer server.Close()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.ERROR), nil)

	resp, err := service.Get(t.Context(), "loop", nil)
	require.Error(t, err)

	if resp != nil {
		resp.Body.Close()
	}

	assert.ErrorContains(t, err, "stopped after 10 redirects")
	// Go refuses the 11th request before it is sent, so the server sees exactly 10.
	assert.EqualValues(t, maxRedirects, requests.Load())
}

// TestKeepAPIKey pins the rule to the one net/http uses for the Authorization
// header: same host or subdomain, port ignored, https never downgrades to http.
func TestKeepAPIKey(t *testing.T) {
	cases := []struct {
		name     string
		initial  string
		dest     string
		expected bool
	}{
		{"same host", "https://api.example.com/a", "https://api.example.com/b", true},
		{"same host, other port", "https://api.example.com:8080/a", "https://api.example.com:8443/b", true},
		{"subdomain", "https://example.com/a", "https://eu.example.com/b", true},
		{"sibling host", "https://api.example.com/a", "https://eu.example.com/b", false},
		{"lookalike host", "https://x.com/a", "https://evilx.com/b", false},
		{"https to http", "https://api.example.com/a", "http://api.example.com/b", false},
		{"ipv6 literal", "https://[::1]/a", "https://[::1]/b", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initial, err := url.Parse(tc.initial)
			require.NoError(t, err)

			dest, err := url.Parse(tc.dest)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, keepAPIKey(dest, initial))
		})
	}
}

func TestRedactAddress(t *testing.T) {
	tests := []struct {
		desc    string
		address string
		exp     string
	}{
		{desc: "no userinfo", address: "http://localhost:9000", exp: "http://localhost:9000"},
		{desc: "no userinfo, with path", address: "https://api.example.com/v1/orders", exp: "https://api.example.com/v1/orders"},
		{desc: "no scheme", address: "localhost:9000", exp: "localhost:9000"},
		{desc: "@ in the path only", address: "http://localhost:9000/users/a@b.com", exp: "http://localhost:9000/users/a@b.com"},
		{desc: "user and password", address: "http://admin:hunter2@localhost:9000", exp: "http://REDACTED@localhost:9000"},
		{desc: "user and password, with path", address: "https://admin:hunter2@api.example.com/v1",
			exp: "https://REDACTED@api.example.com/v1"},
		{desc: "token as user", address: "https://ghp_token@api.example.com", exp: "https://REDACTED@api.example.com"},
		{desc: "password with a bad escape", address: "http://admin:hun%zzter2@localhost:9000", exp: "REDACTED@localhost:9000"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.exp, redactAddress(tc.address))
		})
	}
}

func TestHTTPService_createAndSendRequest_RedactsAddressCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The request itself still carries the credentials from the address.
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "admin", user)
		assert.Equal(t, "hunter2", pass)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := server.Listener.Addr().String()

	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", "http://REDACTED@"+host,
		"method", http.MethodGet, "status", fmt.Sprintf("%v", http.StatusOK)).Times(1)

	recorder := tracetest.NewSpanRecorder()

	// The mock logger writes to the stdout it saw when it was built, so the service is built inside.
	logs := testutil.StdoutOutputForFunc(func() {
		service := NewHTTPService("http://admin:hunter2@"+host, logging.NewMockLogger(logging.INFO), metrics).(*httpService)
		service.Tracer = trace.NewTracerProvider(trace.WithSpanProcessor(recorder)).Tracer("gofr-http-client")

		resp, err := service.createAndSendRequest(t.Context(), http.MethodGet, "test-path", nil, nil, nil)
		require.NoError(t, err)

		resp.Body.Close()
	})

	assert.NotContains(t, logs, "hunter2")
	assert.Contains(t, logs, "http://REDACTED@"+host+"/test-path")

	// The request span is joined by the client-trace spans (connect, headers, ...); none may carry the
	// password, in its name or its attributes, plain or base64-encoded as in the Authorization header.
	spanNames := make([]string, 0, len(recorder.Ended()))
	spans := make([]string, 0, len(recorder.Ended()))

	for _, s := range recorder.Ended() {
		spanNames = append(spanNames, s.Name())
		spans = append(spans, fmt.Sprint(s.Name(), s.Attributes()))
	}

	assert.Contains(t, spanNames, "http://REDACTED@"+host+"/test-path")
	assert.NotContains(t, strings.Join(spans, " "), "hunter2")
	assert.NotContains(t, strings.Join(spans, " "), base64.StdEncoding.EncodeToString([]byte("admin:hunter2")))
}

func TestNewHTTPService_RedactedURL(t *testing.T) {
	tests := []struct {
		desc     string
		address  string
		expected string
	}{
		{desc: "no credentials", address: "http://localhost:9000", expected: ""},
		{desc: "user and password", address: "http://admin:hunter2@localhost:9000", expected: "http://REDACTED@localhost:9000"},
		{desc: "token as user", address: "https://ghp_token@api.example.com", expected: "https://REDACTED@api.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			service := NewHTTPService(tc.address, logging.NewMockLogger(logging.INFO), nil).(*httpService)

			assert.Equal(t, tc.expected, service.redactedURL)
			assert.Equal(t, tc.address, service.url)
		})
	}
}

func TestHTTPService_createAndSendRequest_RedactsAddressCredentialsInErrors(t *testing.T) {
	// A listener that is closed at once gives an address nothing answers on.
	server := httptest.NewServer(http.NotFoundHandler())
	host := server.Listener.Addr().String()
	server.Close()

	tests := []struct {
		desc    string
		address string
		secrets []string
	}{
		{desc: "token as user, upstream down", address: "http://ghp_secrettoken@" + host, secrets: []string{"ghp_secrettoken"}},
		{desc: "user and password, upstream down", address: "http://admin:hunter2@" + host,
			secrets: []string{"admin", "hunter2"}},
		{desc: "password with a bad escape", address: "http://admin:hun%zzter2@" + host, secrets: []string{"admin", "hun", "ter2"}},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var err error

			// The mock logger writes to the stdout it saw when it was built, so the service is built inside.
			logs := testutil.StdoutOutputForFunc(func() {
				service := NewHTTPService(tc.address, logging.NewMockLogger(logging.INFO), nil)

				var resp *http.Response

				resp, err = service.Get(t.Context(), "orders", nil) //nolint:bodyclose // resp is nil when err is set.
				assert.Nil(t, resp)
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "REDACTED@")

			for _, secret := range tc.secrets {
				assert.NotContains(t, err.Error(), secret)
				assert.NotContains(t, logs, secret)
			}
		})
	}
}

func TestHTTPService_createAndSendRequest_KeepsErrorsWithoutCredentials(t *testing.T) {
	// A listener that is closed at once gives an address nothing answers on.
	server := httptest.NewServer(http.NotFoundHandler())
	address := server.URL
	server.Close()

	tests := []struct {
		desc   string
		path   string
		expErr string
	}{
		{desc: "path with a bad escape and an @", path: "users/50%zz@b.com",
			expErr: `parse "` + address + `/users/50%zz@b.com": invalid URL escape "%zz"`},
		{desc: "upstream down", path: "orders", expErr: `Get "` + address + `/orders": `},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			service := NewHTTPService(address, logging.NewMockLogger(logging.FATAL), nil)

			resp, err := service.Get(t.Context(), tc.path, nil) //nolint:bodyclose // resp is nil when err is set.

			assert.Nil(t, resp)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expErr)
			assert.NotContains(t, err.Error(), "REDACTED")
		})
	}
}

func TestHTTPService_createAndSendRequest_RedactedErrorKeepsHostAndPath(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	host := server.Listener.Addr().String()
	server.Close()

	tests := []struct {
		desc    string
		address string
		path    string
		expErr  string
	}{
		{desc: "token as user, path with a bad escape and an @", address: "http://ghp_secrettoken@" + host,
			path: "users/50%zz@b.com", expErr: `parse "http://REDACTED@` + host + `/users/50%zz@b.com": invalid URL escape "%zz"`},
		{desc: "token as user, upstream down", address: "http://ghp_secrettoken@" + host,
			path: "orders", expErr: `Get "http://REDACTED@` + host + `/orders": `},
		{desc: "user and password, upstream down", address: "http://admin:hunter2@" + host,
			path: "orders", expErr: `Get "http://REDACTED@` + host + `/orders": `},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			service := NewHTTPService(tc.address, logging.NewMockLogger(logging.FATAL), nil)

			resp, err := service.Get(t.Context(), tc.path, nil) //nolint:bodyclose // resp is nil when err is set.

			assert.Nil(t, resp)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expErr)
		})
	}
}
