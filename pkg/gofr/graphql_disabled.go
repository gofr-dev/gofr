//go:build gofr_nographql

package gofr

import (
	"errors"
	"net/http"

	"gofr.dev/pkg/gofr/container"
)

// disabledGraphQL stands in for the GraphQL subsystem in a build made with
// -tags gofr_nographql, which omits graphql-go and gqlparser.
//
// GraphQLQuery and GraphQLMutation keep their signatures, so a user's code still
// compiles unchanged; what differs is that registering a resolver logs an error
// naming the tag, and no GraphQL route is served. A service that does not use
// GraphQL is unaffected either way.
type disabledGraphQL struct{ c *container.Container }

const graphQLDisabledMsg = "GraphQL resolver %q was registered, but this binary was built with " +
	"-tags gofr_nographql, which omits the GraphQL engine. Rebuild without the tag to use it."

func newGraphQLRunner(c *container.Container) graphQLRunner {
	return &disabledGraphQL{c: c}
}

// enabled is false, which is what keeps App from registering /graphql and the
// playground route against a handler this build cannot provide.
func (*disabledGraphQL) enabled() bool { return false }

func (d *disabledGraphQL) RegisterQuery(name string, _ Handler) {
	d.c.Logger.Errorf(graphQLDisabledMsg, name)
}

func (d *disabledGraphQL) RegisterMutation(name string, _ Handler) {
	d.c.Logger.Errorf(graphQLDisabledMsg, name)
}

// buildSchema is never reached -- App checks enabled() first -- and reports
// success so that no path can turn a missing engine into a Fatalf.
func (*disabledGraphQL) buildSchema() error { return nil }

// GetHandler is never routed, for the same reason.
func (*disabledGraphQL) GetHandler() http.Handler { return nil }

var errGraphQLDisabled = errors.New("this binary was built with -tags gofr_nographql, which omits the GraphQL engine")

// playgroundHandler exists so gofr.go compiles; App does not route it in this
// build.
func playgroundHandler(_ *Context) (any, error) {
	return nil, errGraphQLDisabled
}
