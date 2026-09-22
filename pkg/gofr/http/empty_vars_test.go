package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// TestPathParamsSurviveEmptyVarsSkip is the compatibility proof for skipping
// mux.SetURLVars when a route has no path parameters.
//
// gorilla/mux allocates a Vars map on every successful match, so a route with no
// parameters used to store an empty map in the request context. Skipping that
// leaves mux.Vars(r) nil for those routes, and this pins that every way GoFr and
// user code reads it still behaves: parameterised routes keep every value, and
// parameter-free routes read as empty rather than panicking.
func TestPathParamsSurviveEmptyVarsSkip(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	tests := []struct {
		name    string
		pattern string
		request string
		want    map[string]string
	}{
		{"no parameters", "/plain", "/plain", map[string]string{}},
		{"one parameter", "/user/{id}", "/user/42", map[string]string{"id": "42"}},
		{"two parameters", "/a/{x}/b/{y}", "/a/1/b/2", map[string]string{"x": "1", "y": "2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]string

			r := NewRouter()
			r.NewRoute().Methods(http.MethodGet).Path(tt.pattern).
				Handler(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
					got = mux.Vars(req)
				}))

			r.ServeHTTP(httptest.NewRecorder(),
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.request, http.NoBody))

			if len(got) != len(tt.want) {
				t.Fatalf("mux.Vars has %d entries, want %d: %v", len(got), len(tt.want), got)
			}

			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("mux.Vars[%q] = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}

// TestNilVarsReadsAreSafe pins the operations GoFr and user handlers perform on
// the vars map, against the nil a parameter-free route now yields. Indexing,
// len, comma-ok and range must all behave as they did against an empty map --
// anything else would be a panic in a running service.
func TestNilVarsReadsAreSafe(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	r := NewRouter()
	r.NewRoute().Methods(http.MethodGet).Path("/plain").
		Handler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			vars := mux.Vars(req)

			if v := vars["absent"]; v != "" {
				t.Errorf("indexing yielded %q, want the zero value", v)
			}

			if v, ok := vars["absent"]; ok || v != "" {
				t.Errorf("comma-ok yielded (%q, %v), want (\"\", false)", v, ok)
			}

			if n := len(vars); n != 0 {
				t.Errorf("len is %d, want 0", n)
			}

			for k := range vars {
				t.Errorf("range yielded key %q over an empty map", k)
			}

			w.WriteHeader(http.StatusOK)
		}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/plain", http.NoBody))

	if w.Code != http.StatusOK {
		t.Errorf("status %d, want 200", w.Code)
	}
}

// TestEmptyVarsSkipMatchesMuxBehaviour is the differential guard: the value a
// handler reads for a path parameter must be identical under both matchers, so
// switching GOFR_ROUTER can never change what a handler sees.
func TestEmptyVarsSkipMatchesMuxBehaviour(t *testing.T) {
	for _, pattern := range []string{"/plain", "/user/{id}", "/a/{x}/b/{y}"} {
		request := map[string]string{
			"/plain":       "/plain",
			"/user/{id}":   "/user/42",
			"/a/{x}/b/{y}": "/a/1/b/2",
		}[pattern]

		results := make(map[string]map[string]string, 2)

		for _, matcher := range []string{MatcherMux, MatcherTrie} {
			t.Setenv(RouterEnvVar, matcher)

			var got map[string]string

			r := NewRouter()
			r.NewRoute().Methods(http.MethodGet).Path(pattern).
				Handler(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
					got = mux.Vars(req)
				}))

			r.ServeHTTP(httptest.NewRecorder(),
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, request, http.NoBody))

			results[matcher] = got
		}

		muxVars, trieVars := results[MatcherMux], results[MatcherTrie]

		if len(muxVars) != len(trieVars) {
			t.Errorf("%s: mux gave %d vars, trie gave %d", pattern, len(muxVars), len(trieVars))
			continue
		}

		for k, v := range muxVars {
			if trieVars[k] != v {
				t.Errorf("%s: vars[%q] is %q under mux and %q under trie",
					pattern, k, v, trieVars[k])
			}
		}
	}
}
