//go:build !gofr_nogrpc

package gofr

import (
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/container"
	grpcMiddleware "gofr.dev/pkg/gofr/grpc/middleware"
	"gofr.dev/pkg/gofr/http/middleware"
)

// The gRPC half of EnableBasicAuth, EnableAPIKeyAuth and EnableOAuth.
//
// It lives here, rather than in auth.go beside the HTTP half, because building
// an interceptor names gofr.dev/pkg/gofr/grpc/middleware, which imports
// google.golang.org/grpc. auth.go is not tagged -- HTTP auth works in every
// build -- so it must not name those types. What crosses the boundary is the
// credentials themselves, which are plain Go values.
//
// grpc_auth_disabled.go holds the no-op counterparts.

// addGRPCBasicAuth registers basic-auth interceptors on the gRPC server. Exactly
// one of the three validation modes is set by the caller; BasicAuthProvider
// picks between them the same way the HTTP provider does.
func (a *App) addGRPCBasicAuth(users map[string]string,
	validateFunc func(username, password string) bool,
	validateFuncWithDatasources func(c *container.Container, username, password string) bool,
) {
	a.addGRPCInterceptors(func() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
		p := grpcMiddleware.BasicAuthProvider{
			Users:                       users,
			ValidateFunc:                validateFunc,
			ValidateFuncWithDatasources: validateFuncWithDatasources,
			Container:                   a.container,
		}

		return grpcMiddleware.BasicAuthUnaryInterceptor(p), grpcMiddleware.BasicAuthStreamInterceptor(p)
	})
}

// addGRPCAPIKeyAuth registers API-key interceptors on the gRPC server.
func (a *App) addGRPCAPIKeyAuth(apiKeys []string,
	validateFunc func(apiKey string) bool,
	validateFuncWithDatasources func(c *container.Container, apiKey string) bool,
) {
	a.addGRPCInterceptors(func() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
		p := grpcMiddleware.APIKeyAuthProvider{
			APIKeys:                     apiKeys,
			ValidateFunc:                validateFunc,
			ValidateFuncWithDatasources: validateFuncWithDatasources,
			Container:                   a.container,
		}

		return grpcMiddleware.APIKeyAuthUnaryInterceptor(p), grpcMiddleware.APIKeyAuthStreamInterceptor(p)
	})
}

// addGRPCOAuth registers OAuth interceptors on the gRPC server. The public-key
// provider is an HTTP-middleware type shared with the gRPC interceptors, so it
// crosses the boundary unchanged.
func (a *App) addGRPCOAuth(publicKeyProvider middleware.PublicKeyProvider, options ...jwt.ParserOption) {
	a.addGRPCInterceptors(func() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
		return grpcMiddleware.OAuthUnaryInterceptor(publicKeyProvider, options...),
			grpcMiddleware.OAuthStreamInterceptor(publicKeyProvider, options...)
	})
}

// addGRPCInterceptors builds the pair only when there is a real gRPC server to
// take them, so a gRPC-free app pays nothing for an auth call it made for HTTP.
func (a *App) addGRPCInterceptors(build func() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor)) {
	g, ok := a.grpcServer.(*grpcServer)
	if !ok || g == nil {
		return
	}

	unary, stream := build()

	g.addUnaryInterceptors(unary)
	g.addStreamInterceptors(stream)
}
