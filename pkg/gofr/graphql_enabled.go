//go:build !gofr_nographql

package gofr

import (
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/response"
)

// graphQLLinked reports whether the GraphQL engine is compiled into this binary.
// See the comment on graphQLRunner for why this is a constant rather than a
// method, and pubsubBackendsLinked for the same pattern on the pub/sub side.
const graphQLLinked = true

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
