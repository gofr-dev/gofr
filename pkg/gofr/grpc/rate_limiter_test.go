package grpc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	httpmw "gofr.dev/pkg/gofr/http/middleware"
)

var (
	errStoreFailure = errors.New("store failure")
	errRedisDown    = errors.New("redis down")
)

type rateLimiterMockMetrics struct {
	mu       sync.Mutex
	counters map[string]int
}

func newRateLimiterMockMetrics() *rateLimiterMockMetrics {
	return &rateLimiterMockMetrics{
		counters: make(map[string]int),
	}
}

func (m *rateLimiterMockMetrics) IncrementCounter(_ context.Context, name string, _ ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[name]++
}

func (*rateLimiterMockMetrics) RecordHistogram(context.Context, string, float64, ...string) {}

func (m *rateLimiterMockMetrics) GetCounter(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.counters[name]
}

type rateLimitMockStream struct {
	grpc.ServerStream
	ctx      context.Context
	headerMD metadata.MD
}

func (s *rateLimitMockStream) Context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}

	return s.ctx
}

func (s *rateLimitMockStream) SetHeader(md metadata.MD) error {
	s.headerMD = md
	return nil
}

func (*rateLimitMockStream) SendMsg(any) error            { return nil }
func (*rateLimitMockStream) RecvMsg(any) error            { return nil }
func (*rateLimitMockStream) SendHeader(metadata.MD) error { return nil }

type fakeStore struct {
	allowed    bool
	retryAfter time.Duration
	err        error
}

func (f *fakeStore) Allow(context.Context, string, httpmw.RateLimiterConfig) (bool, time.Duration, error) {
	return f.allowed, f.retryAfter, f.err
}

func (*fakeStore) StartCleanup(context.Context) {}
func (*fakeStore) StopCleanup()                 {}

type fakeAddr string

func (a fakeAddr) Network() string {
	_ = a

	return "tcp"
}
func (a fakeAddr) String() string  { return string(a) }

func Test_first(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{name: "nil slice", vals: nil, want: ""},
		{name: "empty slice", vals: []string{}, want: ""},
		{name: "single element", vals: []string{"a"}, want: "a"},
		{name: "multiple elements", vals: []string{"a", "b", "c"}, want: "a"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, first(tc.vals))
		})
	}
}

func Test_normalizeIP(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "valid IPv4", input: "10.0.0.1", want: "10.0.0.1"},
		{name: "IPv4 with port", input: "10.0.0.1:8080", want: "10.0.0.1"},
		{name: "valid IPv6 loopback", input: "::1", want: "::1"},
		{name: "bracketed IPv6", input: "[::1]", want: "::1"},
		{name: "IPv6 with port", input: "[::1]:8080", want: "::1"},
		{name: "invalid IP", input: "not-an-ip", want: ""},
		{name: "full IPv6 address", input: "2001:db8::1", want: "2001:db8::1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeIP(tc.input))
		})
	}
}

func Test_getForwardedIP(t *testing.T) {
	tests := []struct {
		name string
		md   metadata.MD
		want string
	}{
		{name: "no metadata", md: nil, want: ""},
		{name: "no xff header", md: metadata.MD{}, want: ""},
		{name: "empty xff value", md: metadata.Pairs("x-forwarded-for", ""), want: ""},
		{name: "single valid IP", md: metadata.Pairs("x-forwarded-for", "10.0.0.1"), want: "10.0.0.1"},
		{name: "multiple IPs takes first", md: metadata.Pairs("x-forwarded-for", "10.0.0.1, 10.0.0.2, 10.0.0.3"), want: "10.0.0.1"},
		{name: "invalid IP returns empty", md: metadata.Pairs("x-forwarded-for", "bad-ip"), want: ""},
		{name: "whitespace trimmed", md: metadata.Pairs("x-forwarded-for", "  10.0.0.1  "), want: "10.0.0.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tc.md)
			}

			assert.Equal(t, tc.want, getForwardedIP(ctx))
		})
	}
}

func Test_getRealIP(t *testing.T) {
	tests := []struct {
		name string
		md   metadata.MD
		want string
	}{
		{name: "no metadata", md: nil, want: ""},
		{name: "no x-real-ip header", md: metadata.MD{}, want: ""},
		{name: "valid IP", md: metadata.Pairs("x-real-ip", "10.0.0.5"), want: "10.0.0.5"},
		{name: "whitespace trimmed", md: metadata.Pairs("x-real-ip", "  10.0.0.5  "), want: "10.0.0.5"},
		{name: "invalid IP returns empty", md: metadata.Pairs("x-real-ip", "bogus"), want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tc.md)
			}

			assert.Equal(t, tc.want, getRealIP(ctx))
		})
	}
}

func Test_getIP_XForwardedFor(t *testing.T) {
	md := metadata.Pairs("x-forwarded-for", "203.0.113.1, 198.51.100.1")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	ctx = peer.NewContext(ctx, &peer.Peer{Addr: fakeAddr("192.168.1.1:12345")})

	ip := getIP(ctx, true)
	assert.Equal(t, "203.0.113.1", ip, "Should extract first IP from X-Forwarded-For when trusting proxies")

	ip = getIP(ctx, false)
	assert.Equal(t, "192.168.1.1", ip, "Should use peer addr when not trusting proxies")
}

func Test_getIP_XRealIP(t *testing.T) {
	md := metadata.Pairs("x-real-ip", "203.0.113.5")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	ctx = peer.NewContext(ctx, &peer.Peer{Addr: fakeAddr("192.168.1.1:12345")})

	ip := getIP(ctx, true)
	assert.Equal(t, "203.0.113.5", ip, "Should extract IP from X-Real-IP when trusting proxies")

	ip = getIP(ctx, false)
	assert.Equal(t, "192.168.1.1", ip, "Should use peer addr when not trusting proxies")
}

func Test_getIP_Priority(t *testing.T) {
	md := metadata.Pairs("x-forwarded-for", "203.0.113.1", "x-real-ip", "203.0.113.2")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	ctx = peer.NewContext(ctx, &peer.Peer{Addr: fakeAddr("192.168.1.1:12345")})

	ip := getIP(ctx, true)
	assert.Equal(t, "203.0.113.1", ip, "X-Forwarded-For should have highest priority")
}

func Test_getIP_PeerAddr(t *testing.T) {
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: fakeAddr("192.168.1.1:12345")})

	ip := getIP(ctx, false)
	assert.Equal(t, "192.168.1.1", ip, "Should extract IP from peer address")
}

func Test_getIP_PeerAddrWithoutPort(t *testing.T) {
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: fakeAddr("192.168.1.1")})

	ip := getIP(ctx, false)
	assert.Equal(t, "192.168.1.1", ip, "Should handle peer address without port")
}

func Test_getIP_NoPeer(t *testing.T) {
	ip := getIP(context.Background(), false)
	assert.Empty(t, ip, "Should return empty when no peer in context")
}

func Test_getIP_NilPeerAddr(t *testing.T) {
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: nil})

	ip := getIP(ctx, false)
	assert.Empty(t, ip, "Should return empty when peer address is nil")
}

func Test_retryAfterSeconds(t *testing.T) {
	tests := []struct {
		name string
		dur  float64
		want string
	}{
		{name: "zero returns 1", dur: 0, want: "1"},
		{name: "sub-second rounds up", dur: 0.3, want: "1"},
		{name: "exactly 1", dur: 1.0, want: "1"},
		{name: "fractional rounds up", dur: 2.1, want: "3"},
		{name: "whole number", dur: 5.0, want: "5"},
		{name: "negative returns 1", dur: -2, want: "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, retryAfterSeconds(tc.dur))
		})
	}
}

func TestUnaryRateLimitInterceptor_PanicsOnInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config httpmw.RateLimiterConfig
	}{
		{
			name:   "zero values",
			config: httpmw.RateLimiterConfig{},
		},
		{
			name:   "negative RequestsPerSecond",
			config: httpmw.RateLimiterConfig{RequestsPerSecond: -1, Burst: 5},
		},
		{
			name:   "zero Burst",
			config: httpmw.RateLimiterConfig{RequestsPerSecond: 10, Burst: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Panics(t, func() {
				UnaryRateLimitInterceptor(tc.config, nil)
			})
		})
	}
}

func TestUnaryRateLimitInterceptor_DefaultStore(t *testing.T) {
	cfg := httpmw.RateLimiterConfig{RequestsPerSecond: 10, Burst: 5}
	interceptor := UnaryRateLimitInterceptor(cfg, nil)
	require.NotNil(t, interceptor)
}

func TestUnaryRateLimitInterceptor_GlobalLimit(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             false,
	}

	interceptor := UnaryRateLimitInterceptor(config, metrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
	handler := func(_ context.Context, _ any) (any, error) { return "ok", nil }

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), nil)

	for i := 0; i < 2; i++ {
		resp, err := interceptor(ctx, "req", info, handler)
		require.NoError(t, err, "Request %d should succeed", i+1)
		assert.Equal(t, "ok", resp)
	}

	resp, err := interceptor(ctx, "req", info, handler)
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Equal(t, 1, metrics.GetCounter("app_grpc_rate_limit_exceeded_total"))
}

func TestUnaryRateLimitInterceptor_PerIPLimit(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
		TrustedProxies:    true,
	}

	interceptor := UnaryRateLimitInterceptor(config, metrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
	handler := func(_ context.Context, _ any) (any, error) { return "ok", nil }

	ctxForIP := func(ip string) context.Context {
		md := metadata.Pairs("x-forwarded-for", ip)
		ctx := metadata.NewIncomingContext(context.Background(), md)

		return grpc.NewContextWithServerTransportStream(ctx, nil)
	}

	for i := 0; i < 2; i++ {
		resp, err := interceptor(ctxForIP("10.0.0.1"), "req", info, handler)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp)
	}

	_, err := interceptor(ctxForIP("10.0.0.1"), "req", info, handler)
	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())

	resp, err := interceptor(ctxForIP("10.0.0.2"), "req", info, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryRateLimitInterceptor_EmptyIPFallback(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
		TrustedProxies:    false,
	}

	interceptor := UnaryRateLimitInterceptor(config, metrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
	handler := func(_ context.Context, _ any) (any, error) { return "ok", nil }

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), nil)

	for i := 0; i < 2; i++ {
		resp, err := interceptor(ctx, "req", info, handler)
		require.NoError(t, err, "Request %d should succeed under 'unknown' key", i+1)
		assert.Equal(t, "ok", resp)
	}

	_, err := interceptor(ctx, "req", info, handler)
	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestUnaryRateLimitInterceptor_StoreError_FailsOpen(t *testing.T) {
	store := &fakeStore{err: errStoreFailure}
	cfg := httpmw.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             5,
		Store:             store,
	}

	interceptor := UnaryRateLimitInterceptor(cfg, nil)

	resp, err := interceptor(context.Background(), "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(context.Context, any) (any, error) { return "ok", nil })

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryRateLimitInterceptor_DeniedNilMetrics(t *testing.T) {
	store := &fakeStore{allowed: false, retryAfter: 2 * time.Second}
	cfg := httpmw.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             5,
		Store:             store,
	}

	interceptor := UnaryRateLimitInterceptor(cfg, nil)

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), nil)

	resp, err := interceptor(ctx, "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(context.Context, any) (any, error) { return "no", nil })

	assert.Nil(t, resp)
	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestUnaryRateLimitInterceptor_TokenRefill(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 5,
		Burst:             2,
		PerIP:             false,
	}

	interceptor := UnaryRateLimitInterceptor(config, metrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
	handler := func(_ context.Context, _ any) (any, error) { return "ok", nil }

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), nil)

	for i := 0; i < 2; i++ {
		resp, err := interceptor(ctx, "req", info, handler)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp)
	}

	_, err := interceptor(ctx, "req", info, handler)
	require.Error(t, err)

	time.Sleep(220 * time.Millisecond)

	resp, err := interceptor(ctx, "req", info, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryRateLimitInterceptor_ConcurrentRequests(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             10,
		PerIP:             true,
		TrustedProxies:    true,
	}

	interceptor := UnaryRateLimitInterceptor(config, metrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
	handler := func(_ context.Context, _ any) (any, error) { return "ok", nil }

	var (
		wg               sync.WaitGroup
		mu               sync.Mutex
		successCount     int
		rateLimitedCount int
	)

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			md := metadata.Pairs("x-forwarded-for", "192.168.1.1")
			ctx := metadata.NewIncomingContext(context.Background(), md)
			ctx = grpc.NewContextWithServerTransportStream(ctx, nil)

			_, err := interceptor(ctx, "req", info, handler)

			mu.Lock()
			defer mu.Unlock()

			if err == nil {
				successCount++
			} else {
				rateLimitedCount++
			}
		}()
	}

	wg.Wait()

	assert.GreaterOrEqual(t, successCount, 9, "Should allow approximately burst size requests")
	assert.LessOrEqual(t, successCount, 11, "Should not allow significantly more than burst size")
	assert.Positive(t, rateLimitedCount, "Should have some rate limited requests")
	assert.Equal(t, 20, successCount+rateLimitedCount, "Total requests should be 20")
}

func TestUnaryRateLimitInterceptor_TrustedProxiesDisabled(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
		TrustedProxies:    false,
	}

	interceptor := UnaryRateLimitInterceptor(config, metrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
	handler := func(_ context.Context, _ any) (any, error) { return "ok", nil }

	makeCtx := func(spoofedIP string) context.Context {
		md := metadata.Pairs("x-forwarded-for", spoofedIP)
		ctx := metadata.NewIncomingContext(context.Background(), md)
		ctx = peer.NewContext(ctx, &peer.Peer{Addr: fakeAddr("127.0.0.1:12345")})

		return grpc.NewContextWithServerTransportStream(ctx, nil)
	}

	for i := 0; i < 2; i++ {
		resp, err := interceptor(makeCtx("203.0.113."+string(rune('1'+i))), "req", info, handler)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp)
	}

	_, err := interceptor(makeCtx("203.0.113.99"), "req", info, handler)
	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestUnaryRateLimitInterceptor_RetryAfterHeader(t *testing.T) {
	store := &fakeStore{allowed: false, retryAfter: 3 * time.Second}
	cfg := httpmw.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             5,
		Store:             store,
	}

	interceptor := UnaryRateLimitInterceptor(cfg, nil)

	ctx := grpc.NewContextWithServerTransportStream(context.Background(), nil)

	_, err := interceptor(ctx, "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(context.Context, any) (any, error) { return "no", nil })

	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestStreamRateLimitInterceptor_PanicsOnInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config httpmw.RateLimiterConfig
	}{
		{
			name:   "zero values",
			config: httpmw.RateLimiterConfig{},
		},
		{
			name:   "negative RequestsPerSecond",
			config: httpmw.RateLimiterConfig{RequestsPerSecond: -1, Burst: 5},
		},
		{
			name:   "zero Burst",
			config: httpmw.RateLimiterConfig{RequestsPerSecond: 10, Burst: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Panics(t, func() {
				StreamRateLimitInterceptor(tc.config, nil)
			})
		})
	}
}

func TestStreamRateLimitInterceptor_DefaultStore(t *testing.T) {
	cfg := httpmw.RateLimiterConfig{RequestsPerSecond: 10, Burst: 5}
	interceptor := StreamRateLimitInterceptor(cfg, nil)
	require.NotNil(t, interceptor)
}

func TestStreamRateLimitInterceptor_GlobalLimit(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             false,
	}

	interceptor := StreamRateLimitInterceptor(config, metrics)
	info := &grpc.StreamServerInfo{FullMethod: "/svc/Stream"}
	handler := func(any, grpc.ServerStream) error { return nil }

	for i := 0; i < 2; i++ {
		ss := &rateLimitMockStream{ctx: context.Background()}
		err := interceptor(nil, ss, info, handler)
		require.NoError(t, err, "Request %d should succeed", i+1)
	}

	ss := &rateLimitMockStream{ctx: context.Background()}
	err := interceptor(nil, ss, info, handler)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Equal(t, 1, metrics.GetCounter("app_grpc_rate_limit_exceeded_total"))
}

func TestStreamRateLimitInterceptor_PerIPLimit(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
		TrustedProxies:    true,
	}

	interceptor := StreamRateLimitInterceptor(config, metrics)
	info := &grpc.StreamServerInfo{FullMethod: "/svc/Stream"}
	handler := func(any, grpc.ServerStream) error { return nil }

	streamForIP := func(ip string) *rateLimitMockStream {
		md := metadata.Pairs("x-forwarded-for", ip)
		ctx := metadata.NewIncomingContext(context.Background(), md)

		return &rateLimitMockStream{ctx: ctx}
	}

	for i := 0; i < 2; i++ {
		err := interceptor(nil, streamForIP("10.0.0.1"), info, handler)
		require.NoError(t, err)
	}

	err := interceptor(nil, streamForIP("10.0.0.1"), info, handler)
	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())

	err = interceptor(nil, streamForIP("10.0.0.2"), info, handler)
	require.NoError(t, err)
}

func TestStreamRateLimitInterceptor_EmptyIPFallback(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
		TrustedProxies:    false,
	}

	interceptor := StreamRateLimitInterceptor(config, metrics)
	info := &grpc.StreamServerInfo{FullMethod: "/svc/Stream"}
	handler := func(any, grpc.ServerStream) error { return nil }

	for i := 0; i < 2; i++ {
		ss := &rateLimitMockStream{ctx: context.Background()}
		err := interceptor(nil, ss, info, handler)
		require.NoError(t, err, "Request %d should succeed under 'unknown' key", i+1)
	}

	ss := &rateLimitMockStream{ctx: context.Background()}
	err := interceptor(nil, ss, info, handler)
	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestStreamRateLimitInterceptor_StoreError_FailsOpen(t *testing.T) {
	store := &fakeStore{err: errRedisDown}
	cfg := httpmw.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             5,
		Store:             store,
	}

	interceptor := StreamRateLimitInterceptor(cfg, nil)

	ss := &rateLimitMockStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/svc/Stream"},
		func(any, grpc.ServerStream) error { return nil })

	require.NoError(t, err)
}

func TestStreamRateLimitInterceptor_DeniedNilMetrics(t *testing.T) {
	store := &fakeStore{allowed: false, retryAfter: 1 * time.Second}
	cfg := httpmw.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             5,
		Store:             store,
	}

	interceptor := StreamRateLimitInterceptor(cfg, nil)

	ss := &rateLimitMockStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/svc/Stream"},
		func(any, grpc.ServerStream) error { return nil })

	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestStreamRateLimitInterceptor_RetryAfterHeader(t *testing.T) {
	store := &fakeStore{allowed: false, retryAfter: 5 * time.Second}
	cfg := httpmw.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             5,
		Store:             store,
	}

	metrics := newRateLimiterMockMetrics()
	interceptor := StreamRateLimitInterceptor(cfg, metrics)

	ss := &rateLimitMockStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/svc/Stream"},
		func(any, grpc.ServerStream) error { return nil })

	require.Error(t, err)
	assert.Equal(t, "5", ss.headerMD.Get("retry-after")[0])
	assert.Equal(t, 1, metrics.GetCounter("app_grpc_rate_limit_exceeded_total"))
}

func TestStreamRateLimitInterceptor_TokenRefill(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 5,
		Burst:             2,
		PerIP:             false,
	}

	interceptor := StreamRateLimitInterceptor(config, metrics)
	info := &grpc.StreamServerInfo{FullMethod: "/svc/Stream"}
	handler := func(any, grpc.ServerStream) error { return nil }

	// Exhaust burst
	for i := 0; i < 2; i++ {
		ss := &rateLimitMockStream{ctx: context.Background()}
		err := interceptor(nil, ss, info, handler)
		require.NoError(t, err)
	}

	ss := &rateLimitMockStream{ctx: context.Background()}
	err := interceptor(nil, ss, info, handler)
	require.Error(t, err)

	time.Sleep(220 * time.Millisecond)

	ss = &rateLimitMockStream{ctx: context.Background()}
	err = interceptor(nil, ss, info, handler)
	require.NoError(t, err)
}

func TestStreamRateLimitInterceptor_ConcurrentRequests(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := httpmw.RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             10,
		PerIP:             true,
		TrustedProxies:    true,
	}

	interceptor := StreamRateLimitInterceptor(config, metrics)
	info := &grpc.StreamServerInfo{FullMethod: "/svc/Stream"}

	var (
		wg               sync.WaitGroup
		successCount     int64
		rateLimitedCount int64
	)

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			md := metadata.Pairs("x-forwarded-for", "192.168.1.1")
			ctx := metadata.NewIncomingContext(context.Background(), md)
			ss := &rateLimitMockStream{ctx: ctx}

			err := interceptor(nil, ss, info, func(any, grpc.ServerStream) error {
				return nil
			})

			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&rateLimitedCount, 1)
			}
		}()
	}

	wg.Wait()

	assert.GreaterOrEqual(t, successCount, int64(9), "Should allow approximately burst size requests")
	assert.LessOrEqual(t, successCount, int64(11), "Should not allow significantly more than burst size")
	assert.Positive(t, rateLimitedCount, "Should have some rate limited requests")
	assert.Equal(t, int64(20), successCount+rateLimitedCount, "Total requests should be 20")
}
