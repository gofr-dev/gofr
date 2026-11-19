package gofr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/internal/routemark"
)

// TestHTTPServerSetup_MarksBuiltInRoutes registers GoFr's built-in routes next to application routes
// that share their prefix or template, and asks the question the RBAC middleware will ask: does the
// route that answered this request carry the built-in mark. Both matchers are driven through the
// real router.
func TestHTTPServerSetup_MarksBuiltInRoutes(t *testing.T) {
	setupOpenAPIStaticFile(t, []byte(`{"openapi":"3.0.0"}`))

	testCases := []struct {
		desc string
		path string
		want bool
	}{
		{"health", "/.well-known/health", true},
		{"alive", "/.well-known/alive", true},
		{"favicon", faviconPath, true},
		{"openapi document", "/.well-known/openapi.json", true},
		{"swagger UI", "/.well-known/swagger", true},
		{"swagger UI asset", "/.well-known/swagger-ui.css", true},
		{"application route under /.well-known/", "/.well-known/jwks.json", false},
		{"application route at /", "/", false},
		{"application route", "/hello", false},
		{"catch-all", "/not-registered", false},
	}

	for _, matcher := range []string{"mux", "trie"} {
		t.Run(matcher, func(t *testing.T) {
			t.Setenv("GOFR_ROUTER", matcher)

			app := New()
			app.GET("/.well-known/jwks.json", func(*Context) (any, error) { return "jwks", nil })
			app.GET("/", func(*Context) (any, error) { return "root", nil })
			app.GET("/hello", func(*Context) (any, error) { return "hello", nil })

			answered := map[string]bool{}

			app.httpServer.router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					answered[r.URL.Path] = routemark.BuiltInAnswered(r)
					next.ServeHTTP(w, r)
				})
			})

			app.httpServerSetup()

			for i, tc := range testCases {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, http.NoBody)
				app.httpServer.router.ServeHTTP(httptest.NewRecorder(), req)

				got, ran := answered[tc.path]
				require.True(t, ran, "TEST[%d], Failed.\n%s: the middleware did not run", i, tc.desc)
				assert.Equal(t, tc.want, got, "TEST[%d], Failed.\n%s", i, tc.desc)
			}

			// The marked set is exactly the routes GoFr registered itself. The ./static directory the
			// OpenAPI file lives in is served at /static by New(): a static directory is an application
			// route, so it is not marked.
			marked := 0

			_ = app.httpServer.router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
				tmpl, _ := route.GetPathTemplate()
				isApp := tmpl == "/.well-known/jwks.json" || tmpl == "/" || tmpl == "/hello" ||
					strings.HasPrefix(tmpl, "/static")

				if routemark.IsBuiltIn(route) {
					marked++
				}

				assert.Equal(t, !isApp && !routemark.IsCatchAll(route), routemark.IsBuiltIn(route), "route %q", tmpl)

				return nil
			})

			assert.GreaterOrEqual(t, marked, 6, "health, alive, favicon, openapi, swagger and assets")
		})
	}
}
