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
)

// BasicAuthUnaryInterceptor returns a unary interceptor that validates requests using Basic Authentication.
func BasicAuthUnaryInterceptor(users map[string]string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := validateBasicAuth(ctx, users); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// BasicAuthStreamInterceptor returns a stream interceptor that validates requests using Basic Authentication.
func BasicAuthStreamInterceptor(users map[string]string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := validateBasicAuth(ss.Context(), users); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func validateBasicAuth(ctx context.Context, users map[string]string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Basic <base64>
	parts := strings.SplitN(authHeader[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
		return status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	payload, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid base64 credentials")
	}

	username, password, found := strings.Cut(string(payload), ":")
	if !found {
		return status.Error(codes.Unauthenticated, "invalid credentials format")
	}

	expectedPass, ok := users[username]
	if !ok {
		// Use dummy comparison to prevent timing attacks
		subtle.ConstantTimeCompare([]byte(password), []byte("dummy"))
		return status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if subtle.ConstantTimeCompare([]byte(password), []byte(expectedPass)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return nil
}
