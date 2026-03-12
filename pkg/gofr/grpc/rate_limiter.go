// Package grpc provides gRPC-related additions within the GoFr framework.
//
// # Rate Limiting
//
// The rate limiter interceptors use a token bucket algorithm (via the shared
// RateLimiterStore) to control request throughput for both unary and streaming
// RPCs. Key implementation details:
//
//   - IP extraction priority (when PerIP=true and TrustedProxies=true):
//     X-Forwarded-For (first CSV entry) → X-Real-IP → gRPC peer address.
//   - normalizeIP strips port/bracket notation and validates via net.ParseIP.
//   - Fail-open: if the store returns an error, the request is allowed through
//     to avoid self-inflicted denial of service.
//   - Health check bypass: requests to /grpc.health.v1.Health/* are never
//     rate-limited, preventing probe failures and cascading pod restarts.
//   - retry-after: returned as both gRPC response metadata and an errdetails.RetryInfo
//     proto in the status details. The unary path uses grpc.SendHeader; the stream
//     path uses ss.SendHeader to ensure the header is actually delivered.
package grpc

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	httpmw "gofr.dev/pkg/gofr/http/middleware"
)

const (
	rateLimitKeyGlobal      = "global"
	rateLimitKeyUnknown     = "unknown"
	grpcHealthServicePrefix = "/grpc.health.v1.Health/"
)

// normalizeIP strips port and bracket notation, then validates via net.ParseIP.
// It returns the canonical string form or "" if the input is not a valid IP.
func normalizeIP(s string) string {
	if s == "" {
		return ""
	}

	// Fast-path: try SplitHostPort only when a colon is present.
	// Pure IPv4 without port (e.g. "10.0.0.1") has no colon; skip the overhead.
	if strings.ContainsRune(s, ':') {
		if host, _, err := net.SplitHostPort(s); err == nil {
			s = host
		}
	}

	// Strip residual brackets from bare bracketed IPv6 like "[::1]".
	if len(s) > 1 && s[0] == '[' {
		s = s[1 : len(s)-1]
	}

	ip := net.ParseIP(s)
	if ip == nil {
		return ""
	}

	return ip.String()
}

// extractHeaderIP reads a single metadata header value from the incoming context
// and returns the normalized IP. For X-Forwarded-For it takes only the first
// comma-separated entry (the original client IP).
func extractHeaderIP(md metadata.MD, key string, firstCSV bool) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}

	v := vals[0]
	if v == "" {
		return ""
	}

	if firstCSV {
		if i := strings.IndexByte(v, ','); i >= 0 {
			v = v[:i]
		}
	}

	return normalizeIP(strings.TrimSpace(v))
}

func getIP(ctx context.Context, trustProxy bool) string {
	if trustProxy {
		return getIPFromProxy(ctx)
	}

	return getIPFromPeer(ctx)
}

func getIPFromProxy(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return getIPFromPeer(ctx)
	}

	if ip := extractHeaderIP(md, "x-forwarded-for", true); ip != "" {
		return ip
	}

	if ip := extractHeaderIP(md, "x-real-ip", false); ip != "" {
		return ip
	}

	return getIPFromPeer(ctx)
}

func getIPFromPeer(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}

	return normalizeIP(p.Addr.String())
}

func retryAfterSeconds(durSeconds float64) string {
	secs := math.Max(1, math.Ceil(durSeconds))
	return fmt.Sprintf("%.0f", secs)
}

// buildRateLimitStatus constructs the gRPC status with RetryInfo details.
func buildRateLimitStatus(retryAfter time.Duration) error {
	st := status.New(codes.ResourceExhausted, "rate limit exceeded")
	retryDetail := &errdetails.RetryInfo{
		RetryDelay: durationpb.New(retryAfter),
	}

	st, _ = st.WithDetails(retryDetail)

	return st.Err()
}

// unaryRateLimitExhaustedError sends retry-after header via grpc.SendHeader (works for unary RPCs)
// and returns the ResourceExhausted status.
func unaryRateLimitExhaustedError(ctx context.Context, retryAfter time.Duration) error {
	_ = grpc.SendHeader(ctx, metadata.Pairs("retry-after", retryAfterSeconds(retryAfter.Seconds())))

	return buildRateLimitStatus(retryAfter)
}

// streamRateLimitExhaustedError sends retry-after header via ss.SendHeader (required for streams,
// since grpc.SendHeader is a no-op in the stream path) and returns the ResourceExhausted status.
func streamRateLimitExhaustedError(ss grpc.ServerStream, retryAfter time.Duration) error {
	_ = ss.SendHeader(metadata.Pairs("retry-after", retryAfterSeconds(retryAfter.Seconds())))

	return buildRateLimitStatus(retryAfter)
}

// newRateLimiterStore validates the config, initializes a default in-memory store
// if none is provided, and starts the background cleanup goroutine.
func newRateLimiterStore(ctx context.Context, cfg *httpmw.RateLimiterConfig) {
	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("invalid rate limiter config: %v", err))
	}

	if cfg.Store == nil {
		cfg.Store = httpmw.NewMemoryRateLimiterStore(*cfg)
	}

	cfg.Store.StartCleanup(ctx)
}

// resolveRateLimitKey determines the rate limit bucket key based on config.
func resolveRateLimitKey(ctx context.Context, cfg httpmw.RateLimiterConfig) string {
	if !cfg.PerIP {
		return rateLimitKeyGlobal
	}

	key := getIP(ctx, cfg.TrustedProxies)
	if key == "" {
		return rateLimitKeyUnknown
	}

	return key
}

// recordRateLimitViolation logs and increments the counter metric for a rate limit violation.
func recordRateLimitViolation(ctx context.Context, l Logger, m Metrics, key, method, callType string) {
	if l != nil {
		l.Info(fmt.Sprintf("rate limit exceeded for key: %s, method: %s", key, method))
	}

	type counterMetrics interface {
		IncrementCounter(ctx context.Context, name string, labels ...string)
	}

	if cm, ok := m.(counterMetrics); ok {
		cm.IncrementCounter(ctx, "app_grpc_rate_limit_exceeded_total",
			"method", method,
			"type", callType,
		)
	}
}

// UnaryRateLimitInterceptor returns a gRPC unary server interceptor that enforces
// rate limiting using the provided configuration. Pass app.Logger() and app.Metrics()
// for logging and Prometheus counter support.
func UnaryRateLimitInterceptor(
	ctx context.Context, cfg httpmw.RateLimiterConfig, l Logger, m Metrics,
) grpc.UnaryServerInterceptor {
	newRateLimiterStore(ctx, &cfg)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, grpcHealthServicePrefix) {
			return handler(ctx, req)
		}

		key := resolveRateLimitKey(ctx, cfg)

		allowed, retryAfter, err := cfg.Store.Allow(ctx, key, cfg)
		if err != nil {
			return handler(ctx, req)
		}

		if !allowed {
			recordRateLimitViolation(ctx, l, m, key, info.FullMethod, "unary")

			return nil, unaryRateLimitExhaustedError(ctx, retryAfter)
		}

		return handler(ctx, req)
	}
}

// StreamRateLimitInterceptor returns a gRPC stream server interceptor that enforces
// rate limiting on stream creation using the provided configuration. Pass app.Logger()
// and app.Metrics() for logging and Prometheus counter support.
func StreamRateLimitInterceptor(
	ctx context.Context, cfg httpmw.RateLimiterConfig, l Logger, m Metrics,
) grpc.StreamServerInterceptor {
	newRateLimiterStore(ctx, &cfg)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if strings.HasPrefix(info.FullMethod, grpcHealthServicePrefix) {
			return handler(srv, ss)
		}

		streamCtx := ss.Context()

		key := resolveRateLimitKey(streamCtx, cfg)

		allowed, retryAfter, err := cfg.Store.Allow(streamCtx, key, cfg)
		if err != nil {
			return handler(srv, ss)
		}

		if !allowed {
			recordRateLimitViolation(streamCtx, l, m, key, info.FullMethod, "stream")

			return streamRateLimitExhaustedError(ss, retryAfter)
		}

		return handler(srv, ss)
	}
}
