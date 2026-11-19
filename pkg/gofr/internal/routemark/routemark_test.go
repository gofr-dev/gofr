package routemark

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCatchAll(t *testing.T) {
	router := mux.NewRouter()

	testCases := []struct {
		desc  string
		route *mux.Route
		want  bool
	}{
		{"marked route", MarkCatchAll(router.PathPrefix("/")), true},
		{"unmarked prefix route at /", router.PathPrefix("/"), false},
		{"unmarked exact route at /", router.Path("/"), false},
		{"route with another name", router.PathPrefix("/").Name("other"), false},
		{"nil route", nil, false},
	}

	for i, tc := range testCases {
		assert.Equal(t, tc.want, IsCatchAll(tc.route), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestIsBuiltIn(t *testing.T) {
	router := mux.NewRouter()

	testCases := []struct {
		desc  string
		route *mux.Route
		want  bool
	}{
		{"marked route", MarkBuiltIn(router.Path("/.well-known/alive")), true},
		{"second route marked with the same name", MarkBuiltIn(router.Path("/favicon.ico")), true},
		{"unmarked route", router.Path("/.well-known/jwks.json"), false},
		{"catch-all is not built in", MarkCatchAll(router.PathPrefix("/")), false},
		{"named route keeps its name and is not marked", MarkBuiltIn(router.Path("/named").Name("app")), false},
		{"nil route", nil, false},
	}

	for i, tc := range testCases {
		assert.Equal(t, tc.want, IsBuiltIn(tc.route), "TEST[%d], Failed.\n%s", i, tc.desc)

		if tc.route != nil {
			require.NoError(t, tc.route.GetError(), "TEST[%d], Failed.\n%s: marking must not break the route", i, tc.desc)
		}
	}

	assert.Nil(t, MarkBuiltIn(nil), "marking a nil route is a no-op")
}

func TestMatched(t *testing.T) {
	route := mux.NewRouter().Path("/x")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)

	assert.Nil(t, Matched(req), "an unrouted request has no matched route")
	assert.False(t, CatchAllAnswered(req), "an unrouted request was not answered by the catch-all")
	assert.False(t, BuiltInAnswered(req), "an unrouted request was not answered by a built-in route")
	assert.True(t, BuiltInAnswered(With(req, MarkBuiltIn(route))), "a request a marked route matched")
	assert.Same(t, req, With(req, nil), "no route must not copy the request")
	assert.Same(t, route, Matched(With(req, route)), "the route put in the context must come back")
}
