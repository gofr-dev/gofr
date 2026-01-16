package middleware

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	httpMiddleware "gofr.dev/pkg/gofr/http/middleware"
)

const (
	jwtRegexPattern = "^[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+$"
	headerParts     = 2
)

var (
	errKeyNotFound = errors.New("key not found")
)

// OAuthUnaryInterceptor returns a gRPC unary server interceptor that validates the OAuth token.
func OAuthUnaryInterceptor(key httpMiddleware.PublicKeyProvider, options ...jwt.ParserOption) grpc.UnaryServerInterceptor {
	regex := regexp.MustCompile(jwtRegexPattern)

	options = append(options, jwt.WithIssuedAt())

	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		claims, err := validateOAuth(ctx, key, regex, options...)
		if err != nil {
			return nil, err
		}

		newCtx := context.WithValue(ctx, httpMiddleware.JWTClaim, claims)

		return handler(newCtx, req)
	}
}

// OAuthStreamInterceptor returns a gRPC stream server interceptor that validates the OAuth token.
func OAuthStreamInterceptor(key httpMiddleware.PublicKeyProvider, options ...jwt.ParserOption) grpc.StreamServerInterceptor {
	regex := regexp.MustCompile(jwtRegexPattern)

	options = append(options, jwt.WithIssuedAt())

	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		claims, err := validateOAuth(ss.Context(), key, regex, options...)
		if err != nil {
			return err
		}

		// We need to wrap the stream to inject the new context containing the claims.
		wrapped := &wrappedStream{ss, context.WithValue(ss.Context(), httpMiddleware.JWTClaim, claims)}

		return handler(srv, wrapped)
	}
}

func validateOAuth(ctx context.Context, key httpMiddleware.PublicKeyProvider, regex *regexp.Regexp,
	options ...jwt.ParserOption) (jwt.Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Bearer <token>
	parts := strings.SplitN(authHeader[0], " ", headerParts)
	if len(parts) != headerParts || !strings.EqualFold(parts[0], "Bearer") {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	tokenString := parts[1]
	if !regex.MatchString(tokenString) {
		return nil, status.Error(codes.Unauthenticated, "jwt expected")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		kid := token.Header["kid"]

		jwks := key.Get(fmt.Sprint(kid))
		if jwks == nil {
			return nil, errKeyNotFound
		}

		return jwks, nil
	}, options...)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	if !token.Valid {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return token.Claims, nil
}
