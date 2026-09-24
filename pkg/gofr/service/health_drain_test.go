package service

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

// newConnCountingServer starts a server whose handler writes status and body, and counts
// every new TCP connection it accepts.
func newConnCountingServer(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	var newConns atomic.Int64

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConns.Add(1)
		}
	}

	server.Start()
	t.Cleanup(server.Close)

	return server, &newConns
}

func newHealthTestService(t *testing.T, url string, options ...Options) HTTP {
	t.Helper()

	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	svc := NewHTTPService(url, logging.NewMockLogger(logging.INFO), metrics, options...)

	// A private transport keeps the idle pool, and therefore the connection count, to this test.
	if httpSvc := extractHTTPService(svc); httpSvc != nil {
		httpSvc.Client.Transport = http.DefaultTransport.(*http.Transport).Clone()
	}

	return svc
}

func TestHealthCheck_ReusesConnection(t *testing.T) {
	const calls = 10

	testCases := []struct {
		desc    string
		status  int
		body    string
		options []Options
		want    *Health
	}{
		{
			desc:   "default alive endpoint, UP",
			status: http.StatusOK,
			body:   `{"data":{"status":"UP"}}`,
			want:   &Health{Status: serviceUp},
		},
		{
			desc:   "default alive endpoint, non-200 is DOWN",
			status: http.StatusServiceUnavailable,
			body:   `{"error":{"message":"unavailable"}}`,
			want:   &Health{Status: serviceDown, Details: map[string]any{"error": serviceDownReason}},
		},
		{
			desc:    "custom health endpoint, UP",
			status:  http.StatusOK,
			body:    `{"data":{"status":"UP","details":{"sql":{"status":"UP"}}}}`,
			options: []Options{&HealthConfig{HealthEndpoint: ".well-known/health", Timeout: 1}},
			want:    &Health{Status: serviceUp},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			server, newConns := newConnCountingServer(t, tc.status, tc.body)
			svc := newHealthTestService(t, server.URL, tc.options...)

			host := strings.TrimPrefix(server.URL, "http://")

			for range calls {
				got := svc.HealthCheck(t.Context())

				assert.Equal(t, tc.want.Status, got.Status)
				assert.Equal(t, host, got.Details["host"])
				assert.Equal(t, tc.want.Details["error"], got.Details["error"])
			}

			assert.Equal(t, int64(1), newConns.Load(), "%d health checks should share one connection", calls)
		})
	}
}

// TestHealthCheck_DrainIsBounded pins the 256 KiB bound from both sides: a body of exactly the
// bound is drained and its connection reused, a larger one is not read to the end and its
// connection is closed, exactly as before the drain existed. The status is unchanged in both.
func TestHealthCheck_DrainIsBounded(t *testing.T) {
	const calls = 3

	testCases := []struct {
		desc      string
		status    int
		bodySize  int
		want      string
		wantConns int64
	}{
		{desc: "256 KiB body, 200: reused", status: http.StatusOK, bodySize: 256 << 10, want: serviceUp, wantConns: 1},
		{desc: "256 KiB body, 500: reused", status: http.StatusInternalServerError, bodySize: 256 << 10,
			want: serviceDown, wantConns: 1},
		{desc: "1 MiB body, 200: not reused", status: http.StatusOK, bodySize: 1 << 20, want: serviceUp,
			wantConns: calls},
		{desc: "1 MiB body, 500: not reused", status: http.StatusInternalServerError, bodySize: 1 << 20,
			want: serviceDown, wantConns: calls},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			server, newConns := newConnCountingServer(t, tc.status, strings.Repeat("x", tc.bodySize))
			svc := newHealthTestService(t, server.URL, &HealthConfig{HealthEndpoint: ".well-known/alive", Timeout: 1})

			for range calls {
				assert.Equal(t, tc.want, svc.HealthCheck(t.Context()).Status)
			}

			assert.Equal(t, tc.wantConns, newConns.Load())
		})
	}
}

// TestHealthCheck_StalledBodyDoesNotHoldTheCheck covers a backend that answers 200 and then stops
// sending its body. The status is known from the headers, so the check must return promptly rather
// than wait out its 5s timeout draining a body that is not coming: the container's health round
// and inline circuit-breaker recovery both wait on this call.
func TestHealthCheck_StalledBodyDoesNotHoldTheCheck(t *testing.T) {
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()

		<-release
	}))

	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	// No HealthConfig: the default 5s check timeout applies.
	svc := newHealthTestService(t, server.URL)

	done := make(chan *Health, 1)
	start := time.Now()

	go func() { done <- svc.HealthCheck(t.Context()) }()

	select {
	case got := <-done:
		require.NotNil(t, got)
		assert.Equal(t, serviceUp, got.Status)
		assert.Less(t, time.Since(start), time.Second, "a stalled body must not hold the check open")
	case <-time.After(3 * time.Second):
		t.Fatal("health check waited on a stalled body instead of returning")
	}
}

// TestHealthCheck_ConcurrentChecks runs checks in parallel, as the container's health round and
// circuit-breaker recovery do, against one backend that answers promptly and one whose body stalls.
// Run under -race it covers the per-call drain timer and context cancel.
func TestHealthCheck_ConcurrentChecks(t *testing.T) {
	const perService = 10

	release := make(chan struct{})

	stalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()

		<-release
	}))

	t.Cleanup(func() {
		close(release)
		stalled.Close()
	})

	fast, _ := newConnCountingServer(t, http.StatusServiceUnavailable, `{"error":{"message":"unavailable"}}`)

	fastSvc := newHealthTestService(t, fast.URL)
	stalledSvc := newHealthTestService(t, stalled.URL)

	var wg sync.WaitGroup

	start := time.Now()

	for range perService {
		wg.Add(2)

		go func() {
			defer wg.Done()

			assert.Equal(t, serviceDown, fastSvc.HealthCheck(t.Context()).Status)
		}()

		go func() {
			defer wg.Done()

			assert.Equal(t, serviceUp, stalledSvc.HealthCheck(t.Context()).Status)
		}()
	}

	wg.Wait()

	assert.Less(t, time.Since(start), 2*time.Second, "concurrent checks must not wait out a stalled body")
}

// TestHealthCheck_ConcurrentChecksShareConnections runs waves of two concurrent checks on one
// service, the shape of a readiness and a liveness probe fanning out to the same dependency. Two
// concurrent checks fit the transport's default two idle connections per host, so after the first
// wave dials, later waves reuse them. Without the drain every check dials: 10 connections. The
// bound allows one wave's worth of slack, because under heavy load net/http itself may decline to
// pool a connection whose request write it has not yet seen complete (maxWriteWaitBeforeConnReuse).
func TestHealthCheck_ConcurrentChecksShareConnections(t *testing.T) {
	const (
		waves       = 5
		concurrency = 2 // http.DefaultMaxIdleConnsPerHost
	)

	server, newConns := newConnCountingServer(t, http.StatusOK, `{"data":{"status":"UP"}}`)
	svc := newHealthTestService(t, server.URL)

	for range waves {
		var wg sync.WaitGroup

		for range concurrency {
			wg.Add(1)

			go func() {
				defer wg.Done()

				assert.Equal(t, serviceUp, svc.HealthCheck(t.Context()).Status)
			}()
		}

		wg.Wait()
	}

	assert.LessOrEqual(t, newConns.Load(), int64(2*concurrency),
		"%d waves of %d concurrent checks should reuse the first wave's connections", waves, concurrency)
}
