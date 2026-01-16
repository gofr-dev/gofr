package middleware

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	httpMiddleware "gofr.dev/pkg/gofr/http/middleware"
)

// APIKeyAuthUnaryInterceptor returns a gRPC unary server interceptor that validates the API key.
func APIKeyAuthUnaryInterceptor(apiKeys ...string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		apiKey, err := validateAPIKey(ctx, apiKeys)
		if err != nil {
			return nil, err
		}

		newCtx := context.WithValue(ctx, httpMiddleware.APIKey, apiKey)

		return handler(newCtx, req)
	}
}

// APIKeyAuthStreamInterceptor returns a gRPC stream server interceptor that validates the API key.
func APIKeyAuthStreamInterceptor(apiKeys ...string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		apiKey, err := validateAPIKey(ss.Context(), apiKeys)
		if err != nil {
			return err
		}

		wrapped := &wrappedStream{ss, context.WithValue(ss.Context(), httpMiddleware.APIKey, apiKey)}

		return handler(srv, wrapped)
	}
}

func validateAPIKey(ctx context.Context, validKeys []string) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Check for x-api-key
	values, ok := md["x-api-key"]
	if !ok || len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing x-api-key header")
	}

	apiKey := values[0]

	for _, key := range validKeys {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
			return apiKey, nil
		}
	}

	// Constant time compare with dummy key to mitigate timing attacks
	subtle.ConstantTimeCompare([]byte(apiKey), []byte("dummy"))

	return "", status.Error(codes.Unauthenticated, "invalid api key")
}
