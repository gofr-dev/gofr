//go:build gofr_nogrpc

package gofr

import (
	"github.com/golang-jwt/jwt/v5"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
)

// No-op counterparts to grpc_auth_enabled.go for a build made with
// -tags gofr_nogrpc.
//
// There is nothing to warn about here: EnableBasicAuth and friends are how a
// user protects the HTTP server, and that half still runs. Only the gRPC
// interceptors, which this binary has no server to attach to, are skipped.

func (*App) addGRPCBasicAuth(map[string]string,
	func(username, password string) bool,
	func(c *container.Container, username, password string) bool) {
}

func (*App) addGRPCAPIKeyAuth([]string,
	func(apiKey string) bool,
	func(c *container.Container, apiKey string) bool) {
}

func (*App) addGRPCOAuth(middleware.PublicKeyProvider, ...jwt.ParserOption) {}
