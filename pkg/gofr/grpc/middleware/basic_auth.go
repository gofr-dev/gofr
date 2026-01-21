package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/container"
	httpMiddleware "gofr.dev/pkg/gofr/http/middleware"
)

// BasicAuthProvider holds the configuration for basic authentication.
type BasicAuthProvider struct {
	Users                       map[string]string
	ValidateFunc                func(username, password string) bool
	ValidateFuncWithDatasources func(c *container.Container, username, password string) bool
	Container                   *container.Container
}

// BasicAuthUnaryInterceptor returns a gRPC unary server interceptor that validates the Basic Auth credentials.
func BasicAuthUnaryInterceptor(provider BasicAuthProvider) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		username, err := validateBasicAuth(ctx, provider)
		if err != nil {
			return nil, err
		}

		newCtx := context.WithValue(ctx, httpMiddleware.Username, username)

		return handler(newCtx, req)
	}
}

// BasicAuthStreamInterceptor returns a gRPC stream server interceptor that validates the Basic Auth credentials.
func BasicAuthStreamInterceptor(provider BasicAuthProvider) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		username, err := validateBasicAuth(ss.Context(), provider)
		if err != nil {
			return err
		}

		wrapped := &wrappedStream{ss, context.WithValue(ss.Context(), httpMiddleware.Username, username)}

		return handler(srv, wrapped)
	}
}

func validateBasicAuth(ctx context.Context, provider BasicAuthProvider) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Basic <base64>
	parts := strings.SplitN(authHeader[0], " ", 2) //nolint:mnd // 2 is the number of parts in the header
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	payload, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", status.Error(codes.Unauthenticated, "invalid base64 credentials")
	}

	username, password, found := strings.Cut(string(payload), ":")
	if !found {
		return "", status.Error(codes.Unauthenticated, "invalid credentials format")
	}

	if !provider.verifyCredentials(username, password) {
		return "", status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return username, nil
}

func (b BasicAuthProvider) verifyCredentials(username, password string) bool {
	if b.ValidateFuncWithDatasources != nil {
		return b.ValidateFuncWithDatasources(b.Container, username, password)
	}

	if b.ValidateFunc != nil {
		return b.ValidateFunc(username, password)
	}

	expectedPass, ok := b.Users[username]
	if !ok {
		// Use dummy comparison to prevent timing attacks
		subtle.ConstantTimeCompare([]byte(password), []byte("dummy"))

		return false
	}

	return subtle.ConstantTimeCompare([]byte(password), []byte(expectedPass)) == 1
}
