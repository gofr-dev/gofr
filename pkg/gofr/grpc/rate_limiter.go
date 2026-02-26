package grpc

import (
	"context"
	"fmt"
	"net"
	"strings"

	"gofr.dev/pkg/gofr/http/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func getForwardedIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	forwarded := first(md.Get("x-forwarded-for"))
	if forwarded == "" {
		return ""
	}

	parts := strings.Split(forwarded, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func getRealIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	return strings.TrimSpace(first(md.Get("x-real-ip")))
}

/*
If contains more than one IP, just return the first one
*/
func first(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
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

	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {

		return p.Addr.String()
	}

	return host
}

func UnaryRateLimitInterceptor(config middleware.RateLimiterConfig, m Metrics) grpc.UnaryServerInterceptor {
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid rate limiter config: %v", err))
	}
	if config.Store == nil {
		config.Store = middleware.NewMemoryRateLimiterStore(config)
	}
	ctx := context.Background()
	config.Store.StartCleanup(ctx)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		key := "global"
		if config.PerIP {
			key = getIP(ctx, config.TrustedProxies)
			if key == "" {
				key = "unknown"
			}
		}
		_ = config
		_ = m
		_ = info
		return handler(ctx, req)
	}
}
