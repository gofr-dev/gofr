//go:build gofr_nographql

package gofr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The point of this file is that a binary built with -tags gofr_nographql still
// compiles and runs a user's GraphQL calls without panicking or serving a route
// it cannot back. Everything here is unreachable in the default build.

func TestGraphQLDisabled_RegisterDoesNotPanic(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	app := New()

	require.NotPanics(t, func() {
		app.GraphQLQuery("hello", func(_ *Context) (any, error) { return "world", nil })
		app.GraphQLMutation("save", func(_ *Context) (any, error) { return "ok", nil })
	})

	require.NotNil(t, app.graphqlManager, "the runner is still constructed, so the calls have somewhere to land")
	assert.False(t, app.graphqlManager.enabled())
	assert.False(t, app.graphQLActive(), "no GraphQL routes may be registered in this build")
	assert.Nil(t, app.graphqlManager.GetHandler())
	require.NoError(t, app.graphqlManager.buildSchema(), "buildSchema must not turn a missing engine into a Fatalf")
}

// setupGraphQL is the function that would register a nil handler on /graphql if
// enabled() were not consulted, which would panic on the first request.
func TestGraphQLDisabled_SetupRegistersNoRoute(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	app := New()
	app.GraphQLQuery("hello", func(_ *Context) (any, error) { return "world", nil })

	require.NotPanics(t, app.setupGraphQL)

	req := httptest.NewRequest(http.MethodPost, "/graphql", http.NoBody)
	match := &mux.RouteMatch{}
	assert.False(t, app.httpServer.router.Match(req, match), "/graphql must not be routed in this build")
}

func TestGraphQLDisabled_PlaygroundHandlerErrors(t *testing.T) {
	out, err := playgroundHandler(nil)

	assert.Nil(t, out)
	require.ErrorIs(t, err, errGraphQLDisabled)
}
