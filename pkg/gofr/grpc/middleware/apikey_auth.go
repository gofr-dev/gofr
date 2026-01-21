package middleware

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/container"
	httpMiddleware "gofr.dev/pkg/gofr/http/middleware"
)

// APIKeyAuthProvider holds the configuration for API key authentication.
type APIKeyAuthProvider struct {
	APIKeys                     []string
	ValidateFunc                func(apiKey string) bool
	ValidateFuncWithDatasources func(c *container.Container, apiKey string) bool
	Container                   *container.Container
}

// APIKeyAuthUnaryInterceptor returns a gRPC unary server interceptor that validates the API key.
func APIKeyAuthUnaryInterceptor(provider APIKeyAuthProvider) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		apiKey, err := validateAPIKey(ctx, provider)
		if err != nil {
			return nil, err
		}

		newCtx := context.WithValue(ctx, httpMiddleware.APIKey, apiKey)

		return handler(newCtx, req)
	}
}

// APIKeyAuthStreamInterceptor returns a gRPC stream server interceptor that validates the API key.
func APIKeyAuthStreamInterceptor(provider APIKeyAuthProvider) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		apiKey, err := validateAPIKey(ss.Context(), provider)
		if err != nil {
			return err
		}

		wrapped := &wrappedStream{ss, context.WithValue(ss.Context(), httpMiddleware.APIKey, apiKey)}

		return handler(srv, wrapped)
	}
}

func validateAPIKey(ctx context.Context, provider APIKeyAuthProvider) (string, error) {
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

	if !provider.verifyAPIKey(apiKey) {
		return "", status.Error(codes.Unauthenticated, "invalid api key")
	}

	return apiKey, nil
}

func (a APIKeyAuthProvider) verifyAPIKey(apiKey string) bool {
	if a.ValidateFuncWithDatasources != nil {
		return a.ValidateFuncWithDatasources(a.Container, apiKey)
	}

	if a.ValidateFunc != nil {
		return a.ValidateFunc(apiKey)
	}

	for _, key := range a.APIKeys {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
			return true
		}
	}

	// Constant time compare with dummy key to mitigate timing attacks
	subtle.ConstantTimeCompare([]byte(apiKey), []byte("dummy"))

	return false
}
