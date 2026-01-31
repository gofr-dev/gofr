package middleware

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/container"
	auth "gofr.dev/pkg/gofr/http/middleware"
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
	return NewAuthUnaryInterceptor(func(ctx context.Context) (any, error) {
		return validateAPIKey(ctx, provider)
	}, auth.APIKey)
}

// APIKeyAuthStreamInterceptor returns a gRPC stream server interceptor that validates the API key.
func APIKeyAuthStreamInterceptor(provider APIKeyAuthProvider) grpc.StreamServerInterceptor {
	return NewAuthStreamInterceptor(func(ctx context.Context) (any, error) {
		return validateAPIKey(ctx, provider)
	}, auth.APIKey)
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
