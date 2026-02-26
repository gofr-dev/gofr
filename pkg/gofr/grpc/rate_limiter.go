package grpc

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	httpmw "gofr.dev/pkg/gofr/http/middleware"
)

const (
	rateLimitKeyGlobal  = "global"
	rateLimitKeyUnknown = "unknown"
)

func getForwardedIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	xff := first(md.Get("x-forwarded-for"))
	if xff == "" {
		return ""
	}

	parts := strings.Split(xff, ",")
	if len(parts) == 0 {
		return ""
	}

	return normalizeIP(strings.TrimSpace(parts[0]))
}

func getRealIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	return normalizeIP(strings.TrimSpace(first(md.Get("x-real-ip"))))
}

func first(vals []string) string {
	if len(vals) == 0 {
		return ""
	}

	return vals[0]
}


func normalizeIP(s string) string {
	if s == "" {
		return ""
	}

	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}

	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	ip := net.ParseIP(s)
	if ip == nil {
		return ""
	}

	return ip.String()
}

func getIP(ctx context.Context, trustProxy bool) string {
	if trustProxy {
		if ip := getForwardedIP(ctx); ip != "" {
			return ip
		}

		if ip := getRealIP(ctx); ip != "" {
			return ip
		}
	}

	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}

	// p.Addr.String() is often "ip:port"
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return normalizeIP(p.Addr.String())
	}

	return normalizeIP(host)
}

func retryAfterSeconds(durSeconds float64) string {
	secs := math.Max(1, math.Ceil(durSeconds))
	return fmt.Sprintf("%.0f", secs)
}


func UnaryRateLimitInterceptor(config httpmw.RateLimiterConfig, m Metrics) grpc.UnaryServerInterceptor {
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid rate limiter config: %v", err))
	}

	if config.Store == nil {
		config.Store = httpmw.NewMemoryRateLimiterStore(config)
	}

	config.Store.StartCleanup(context.Background())

		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			key := rateLimitKeyGlobal
		if config.PerIP {
			key = getIP(ctx, config.TrustedProxies)
			if key == "" {
					key = rateLimitKeyUnknown
			}
		}

		allowed, retryAfter, err := config.Store.Allow(ctx, key, config)
		if err != nil {
			return handler(ctx, req)
		}

		if !allowed {
			_ = grpc.SendHeader(ctx, metadata.Pairs(
				"retry-after", retryAfterSeconds(retryAfter.Seconds()),
			))

			if m != nil {
				m.IncrementCounter(ctx, "app_grpc_rate_limit_exceeded_total",
					"method", info.FullMethod,
					"type", "unary",
				)
			}

			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		return handler(ctx, req)
	}
}

func StreamRateLimitInterceptor(config httpmw.RateLimiterConfig, m Metrics) grpc.StreamServerInterceptor {
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid rate limiter config: %v", err))
	}

	if config.Store == nil {
		config.Store = httpmw.NewMemoryRateLimiterStore(config)
	}

	config.Store.StartCleanup(context.Background())

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		streamCtx := ss.Context()

		key := rateLimitKeyGlobal
		if config.PerIP {
			key = getIP(streamCtx, config.TrustedProxies)
			if key == "" {
				key = rateLimitKeyUnknown
			}
		}

		allowed, retryAfter, err := config.Store.Allow(streamCtx, key, config)
		if err != nil {
			return handler(srv, ss)
		}

		if !allowed {
			_ = ss.SetHeader(metadata.Pairs(
				"retry-after", retryAfterSeconds(retryAfter.Seconds()),
			))

			if m != nil {
				m.IncrementCounter(streamCtx, "app_grpc_rate_limit_exceeded_total",
					"method", info.FullMethod,
					"type", "stream",
				)
			}

			return status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		return handler(srv, ss)
	}
}