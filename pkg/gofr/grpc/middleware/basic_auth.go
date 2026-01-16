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

	httpMiddleware "gofr.dev/pkg/gofr/http/middleware"
)

// BasicAuthUnaryInterceptor returns a gRPC unary server interceptor that validates the Basic Auth credentials.
func BasicAuthUnaryInterceptor(users map[string]string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		username, err := validateBasicAuth(ctx, users)
		if err != nil {
			return nil, err
		}

		newCtx := context.WithValue(ctx, httpMiddleware.Username, username)

		return handler(newCtx, req)
	}
}

// BasicAuthStreamInterceptor returns a gRPC stream server interceptor that validates the Basic Auth credentials.
func BasicAuthStreamInterceptor(users map[string]string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		username, err := validateBasicAuth(ss.Context(), users)
		if err != nil {
			return err
		}

		wrapped := &wrappedStream{ss, context.WithValue(ss.Context(), httpMiddleware.Username, username)}

		return handler(srv, wrapped)
	}
}

func validateBasicAuth(ctx context.Context, users map[string]string) (string, error) {
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

	expectedPass, ok := users[username]
	if !ok {
		// Use dummy comparison to prevent timing attacks
		subtle.ConstantTimeCompare([]byte(password), []byte("dummy"))
		return "", status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if subtle.ConstantTimeCompare([]byte(password), []byte(expectedPass)) != 1 {
		return "", status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return username, nil
}
