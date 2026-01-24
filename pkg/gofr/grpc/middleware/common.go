package middleware

import (
	"context"

	"google.golang.org/grpc"
)

// headerParts represents the expected number of parts in an authorization header (e.g., "Bearer <token>").
const headerParts = 2

// wrappedStream wraps a grpc.ServerStream to allow overriding the context.
// This is used to inject authentication information (like username or claims) into the stream's context.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context containing authentication information.
func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// AuthValidator defines a function that validates credentials from context and returns the identity and auth method.
type AuthValidator func(ctx context.Context) (any, error)

// NewAuthUnaryInterceptor creates a unary interceptor using the provided validator and auth method.
func NewAuthUnaryInterceptor(validator AuthValidator, method any) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		val, err := validator(ctx)
		if err != nil {
			return nil, err
		}

		return handler(context.WithValue(ctx, method, val), req)
	}
}

// NewAuthStreamInterceptor creates a stream interceptor using the provided validator and auth method.
func NewAuthStreamInterceptor(validator AuthValidator, method any) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		val, err := validator(ss.Context())
		if err != nil {
			return err
		}

		wrapped := &wrappedStream{ss, context.WithValue(ss.Context(), method, val)}

		return handler(srv, wrapped)
	}
}
