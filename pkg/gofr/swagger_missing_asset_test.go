package gofr

import (
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestSwaggerRoutesOnlyOwnEmbeddedAssets(t *testing.T) {
	// The built-in Swagger routes are registered only if the application's
	// static/openapi.json exists. Match that condition without starting a server.
	openAPIPath := filepath.Join("static", OpenAPIJSON)
	require.NoError(t, os.WriteFile(openAPIPath, []byte(`{"openapi":"3.0.0"}`), 0600))
	t.Cleanup(func() {
		require.NoError(t, os.Remove(openAPIPath))
	})

	for _, matcher := range []string{"mux", "trie"} {
		t.Run(matcher, func(t *testing.T) {
			t.Setenv("GOFR_ROUTER", matcher)

			app := New()
			app.httpRegistered = true // Avoid port availability checks during route setup.
			app.checkAndAddOpenAPIDocumentation()

			// The router must not have a Swagger route for unrelated well-known
			// resources, even if a future request or handler could return 404.
			for _, path := range []string{
				"/.well-known/security.txt",
				"/.well-known/assetlinks.json",
				"/.well-known/unknown.css",
				"/.well-known/files.go",
			} {
				t.Run(path, func(t *testing.T) {
					req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
					var match mux.RouteMatch
					require.False(t, app.httpServer.router.Router.Match(req, &match), "Swagger must not own %s", path)

					recorder := httptest.NewRecorder()
					app.httpServer.router.ServeHTTP(recorder, req)
					require.Equal(t, http.StatusNotFound, recorder.Code, path)
				})
			}

			for _, path := range []string{
				"/.well-known/swagger",
				"/.well-known/openapi.json",
			} {
				t.Run(path, func(t *testing.T) {
					recorder := httptest.NewRecorder()
					req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
					app.httpServer.router.ServeHTTP(recorder, req)
					require.Equal(t, http.StatusOK, recorder.Code, path)
				})
			}

			// Check every embedded web asset, not just one CSS/JS example.
			entries, err := fs.ReadDir("static")
			require.NoError(t, err)

			for _, entry := range entries {
				if entry.IsDir() || !isSwaggerUIAsset(entry.Name()) {
					continue
				}

				t.Run(entry.Name(), func(t *testing.T) {
					path := "/.well-known/" + entry.Name()
					recorder := httptest.NewRecorder()
					req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
					app.httpServer.router.ServeHTTP(recorder, req)
					require.Equal(t, http.StatusOK, recorder.Code, path)
					require.Equal(t, mime.TypeByExtension(filepath.Ext(entry.Name())), recorder.Header().Get("Content-Type"))
					require.NotEmpty(t, recorder.Body.Bytes(), path)
				})
			}
		})
	}
}

func TestSwaggerDoesNotShadowApplicationWellKnownRoute(t *testing.T) {
	openAPIPath := filepath.Join("static", OpenAPIJSON)
	require.NoError(t, os.WriteFile(openAPIPath, []byte(`{"openapi":"3.0.0"}`), 0600))
	t.Cleanup(func() {
		require.NoError(t, os.Remove(openAPIPath))
	})

	app := New()
	app.httpRegistered = true
	app.checkAndAddOpenAPIDocumentation()

	app.GET("/.well-known/security.txt", func(c *Context) (any, error) {
		return "application-defined-security-file", nil
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/security.txt", http.NoBody)
	app.httpServer.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "application-defined-security-file")
}
