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

	auth "gofr.dev/pkg/gofr/http/middleware"
)

const (
	jwtRegexPattern = "^[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+$"
)

var errKeyNotFound = errors.New("key not found")

// OAuthUnaryInterceptor returns a gRPC unary server interceptor that validates the OAuth token.
func OAuthUnaryInterceptor(key auth.PublicKeyProvider, options ...jwt.ParserOption) grpc.UnaryServerInterceptor {
	regex := regexp.MustCompile(jwtRegexPattern)

	options = append(options, jwt.WithIssuedAt())

	return NewAuthUnaryInterceptor(func(ctx context.Context) (any, error) {
		return validateOAuth(ctx, key, regex, options...)
	}, auth.JWTClaim)
}

// OAuthStreamInterceptor returns a gRPC stream server interceptor that validates the OAuth token.
func OAuthStreamInterceptor(key auth.PublicKeyProvider, options ...jwt.ParserOption) grpc.StreamServerInterceptor {
	regex := regexp.MustCompile(jwtRegexPattern)

	options = append(options, jwt.WithIssuedAt())

	return NewAuthStreamInterceptor(func(ctx context.Context) (any, error) {
		return validateOAuth(ctx, key, regex, options...)
	}, auth.JWTClaim)
}

func validateOAuth(ctx context.Context, key auth.PublicKeyProvider, regex *regexp.Regexp,
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
