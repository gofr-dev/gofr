package routemark

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
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

func TestMatched(t *testing.T) {
	route := mux.NewRouter().Path("/x")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)

	assert.Nil(t, Matched(req), "an unrouted request has no matched route")
	assert.False(t, CatchAllAnswered(req), "an unrouted request was not answered by the catch-all")
	assert.Same(t, req, With(req, nil), "no route must not copy the request")
	assert.Same(t, route, Matched(With(req, route)), "the route put in the context must come back")
}
