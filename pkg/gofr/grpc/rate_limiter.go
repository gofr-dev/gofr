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

type CounterMetrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}

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
// and returns the normalised IP. For X-Forwarded-For it takes only the first
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

func rateLimitExhaustedError(ctx context.Context, retryAfter time.Duration) error {
	st := status.New(codes.ResourceExhausted, "rate limit exceeded")
	retryDetail := &errdetails.RetryInfo{
		RetryDelay: durationpb.New(retryAfter),
	}

	st, _ = st.WithDetails(retryDetail)

	_ = grpc.SendHeader(ctx, metadata.Pairs("retry-after", retryAfterSeconds(retryAfter.Seconds())))

	return st.Err()
}

func UnaryRateLimitInterceptor(
	ctx context.Context, cfg httpmw.RateLimiterConfig, l Logger, m CounterMetrics,
) grpc.UnaryServerInterceptor {
	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("invalid rate limiter config: %v", err))
	}

	if cfg.Store == nil {
		cfg.Store = httpmw.NewMemoryRateLimiterStore(cfg)
	}

	cfg.Store.StartCleanup(ctx)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, grpcHealthServicePrefix) {
			return handler(ctx, req)
		}

		key := rateLimitKeyGlobal
		if cfg.PerIP {
			key = getIP(ctx, cfg.TrustedProxies)
			if key == "" {
				key = rateLimitKeyUnknown
			}
		}

		allowed, retryAfter, err := cfg.Store.Allow(ctx, key, cfg)
		if err != nil {
			return handler(ctx, req)
		}

		if !allowed {
			if l != nil {
				l.Errorf("rate limit exceeded for key: %s, method: %s", key, info.FullMethod)
			}

			if m != nil {
				m.IncrementCounter(ctx, "app_grpc_rate_limit_exceeded_total",
					"method", info.FullMethod,
					"type", "unary",
				)
			}

			return nil, rateLimitExhaustedError(ctx, retryAfter)
		}

		return handler(ctx, req)
	}
}

func StreamRateLimitInterceptor(
	ctx context.Context, cfg httpmw.RateLimiterConfig, l Logger, m CounterMetrics,
) grpc.StreamServerInterceptor {
	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("invalid rate limiter config: %v", err))
	}

	if cfg.Store == nil {
		cfg.Store = httpmw.NewMemoryRateLimiterStore(cfg)
	}

	cfg.Store.StartCleanup(ctx)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if strings.HasPrefix(info.FullMethod, grpcHealthServicePrefix) {
			return handler(srv, ss)
		}

		streamCtx := ss.Context()

		key := rateLimitKeyGlobal
		if cfg.PerIP {
			key = getIP(streamCtx, cfg.TrustedProxies)
			if key == "" {
				key = rateLimitKeyUnknown
			}
		}

		allowed, retryAfter, err := cfg.Store.Allow(streamCtx, key, cfg)
		if err != nil {
			return handler(srv, ss)
		}

		if !allowed {
			if l != nil {
				l.Errorf("rate limit exceeded for key: %s, method: %s", key, info.FullMethod)
			}

			if m != nil {
				m.IncrementCounter(streamCtx, "app_grpc_rate_limit_exceeded_total",
					"method", info.FullMethod,
					"type", "stream",
				)
			}

			return rateLimitExhaustedError(streamCtx, retryAfter)
		}

		return handler(srv, ss)
	}
}
