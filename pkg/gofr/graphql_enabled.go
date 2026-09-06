//go:build !gofr_nographql

package gofr

import (
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/response"
)

// The default build wires the real manager, so nothing changes for a user who
// does not ask for -tags gofr_nographql.
func newGraphQLRunner(c *container.Container) graphQLRunner {
	return newGraphQLManager(c)
}

// playgroundHandler serves the GraphQL interactive playground UI.
//
// It lives beside the engine because it reads graphiqlHTML, which ships with it.
func playgroundHandler(_ *Context) (any, error) {
	return response.File{Content: []byte(graphiqlHTML), ContentType: "text/html"}, nil
}
