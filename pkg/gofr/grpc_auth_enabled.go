//go:build !gofr_nogrpc

package gofr

import (
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"

	grpcMiddleware "gofr.dev/pkg/gofr/grpc/middleware"
	"gofr.dev/pkg/gofr/http/middleware"
)

// The gRPC half of EnableBasicAuth, EnableAPIKeyAuth and EnableOAuth.
//
// It lives here, rather than in auth.go beside the HTTP half, because building
// an interceptor names gofr.dev/pkg/gofr/grpc/middleware, which imports
// google.golang.org/grpc. auth.go is not tagged -- HTTP auth works in every
// build -- so it must not name those types. What crosses the boundary is the
// HTTP middleware's provider struct, which carries the same credentials and
// validators in plain Go values.
//
// grpc_auth_disabled.go holds the no-op counterparts.

// addGRPCBasicAuth registers basic-auth interceptors on the gRPC server. The
// caller sets exactly one of Users, ValidateFunc and ValidateFuncWithDatasources;
// the container is always the app's, whatever the HTTP half was given.
func (a *App) addGRPCBasicAuth(p middleware.BasicAuthProvider) {
	a.addGRPCInterceptors(func() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
		gp := grpcMiddleware.BasicAuthProvider{
			Users:                       p.Users,
			ValidateFunc:                p.ValidateFunc,
			ValidateFuncWithDatasources: p.ValidateFuncWithDatasources,
			Container:                   a.container,
		}

		return grpcMiddleware.BasicAuthUnaryInterceptor(gp), grpcMiddleware.BasicAuthStreamInterceptor(gp)
	})
}

// addGRPCAPIKeyAuth registers API-key interceptors on the gRPC server. The
// caller sets exactly one of APIKeys, ValidateFunc and ValidateFuncWithDatasources.
func (a *App) addGRPCAPIKeyAuth(p middleware.APIKeyAuthProvider) {
	a.addGRPCInterceptors(func() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
		gp := grpcMiddleware.APIKeyAuthProvider{
			APIKeys:                     p.APIKeys,
			ValidateFunc:                p.ValidateFunc,
			ValidateFuncWithDatasources: p.ValidateFuncWithDatasources,
			Container:                   a.container,
		}

		return grpcMiddleware.APIKeyAuthUnaryInterceptor(gp), grpcMiddleware.APIKeyAuthStreamInterceptor(gp)
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
	g := a.grpcServer
	if g == nil {
		return
	}

	unary, stream := build()

	g.addUnaryInterceptors(unary)
	g.addStreamInterceptors(stream)
}
