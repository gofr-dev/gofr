package middleware

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// APIKeyAuthUnaryInterceptor returns a unary interceptor that validates requests using API Key Authentication.
func APIKeyAuthUnaryInterceptor(apiKeys ...string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := validateAPIKey(ctx, apiKeys); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// APIKeyAuthStreamInterceptor returns a stream interceptor that validates requests using API Key Authentication.
func APIKeyAuthStreamInterceptor(apiKeys ...string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := validateAPIKey(ss.Context(), apiKeys); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func validateAPIKey(ctx context.Context, validKeys []string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Check for x-api-key
	values, ok := md["x-api-key"]
	if !ok || len(values) == 0 {
		return status.Error(codes.Unauthenticated, "missing x-api-key header")
	}

	apiKey := values[0]

	for _, key := range validKeys {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
			return nil
		}
	}

	// Constant time compare with dummy key to mitigate timing attacks
	subtle.ConstantTimeCompare([]byte(apiKey), []byte("dummy"))

	return status.Error(codes.Unauthenticated, "invalid api key")
}
