package gofr

import (
	"bytes"
	"context"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/logging"
)

func setupOpenAPIStaticFile(t *testing.T, content []byte) {
	t.Helper()
	t.Chdir(t.TempDir())

	require.NoError(t, os.Mkdir("static", 0750))
	require.NoError(t, os.WriteFile(filepath.Join("static", OpenAPIJSON), content, 0600))
}

func TestOpenAPIHandler(t *testing.T) {
	openAPIContent := []byte(`{"swagger": "2.0", "info": {"version": "1.0.0", "title": "Sample API"}}`)
	setupOpenAPIStaticFile(t, openAPIContent)

	testContainer, _ := container.NewMockContainer(t)

	ctx := createTestContext(http.MethodGet, "/.well-known/openapi.json", "", nil, testContainer)

	result, err := OpenAPIHandler(ctx)
	if err != nil {
		t.Fatalf("OpenAPIHandler failed: %v", err)
	}

	fileResponse, ok := result.(response.File)
	if !ok {
		t.Errorf("Expected a FileResponse")
	}

	if fileResponse.ContentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", fileResponse.ContentType)
	}

	if !bytes.Equal(fileResponse.Content, openAPIContent) {
		t.Errorf("Expected response content '%s', got '%s'", string(openAPIContent), string(fileResponse.Content))
	}
}

func TestOpenAPIHandler_Error(t *testing.T) {
	t.Chdir(t.TempDir())

	testContainer, _ := container.NewMockContainer(t)

	ctx := createTestContext(http.MethodGet, "/.well-known/openapi.json", "", nil, testContainer)
	ctx.ContextLogger = *logging.NewContextLogger(ctx, testContainer.Logger)

	result, err := OpenAPIHandler(ctx)

	require.Nil(t, result, "Expected result to be nil")
	require.ErrorIs(t, err, os.ErrNotExist, "Expected file not found error")
}

func TestSwaggerHandler(t *testing.T) {
	testContainer, _ := container.NewMockContainer(t)

	tests := []struct {
		desc        string
		fileName    string
		contentType string
	}{
		{"fetch index.html", "", "text/html"},
		{"fetch favicon image", "favicon-16x16.png", "image/png"},
		{"fetch js files", "swagger-ui.js", "text/javascript"},
	}

	for _, tc := range tests {
		testReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/swagger"+"/"+tc.fileName, http.NoBody)
		testReq = mux.SetURLVars(testReq, map[string]string{"name": tc.fileName})
		gofrReq := gofrHTTP.NewRequest(testReq)

		ctx := newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet), gofrReq, testContainer)

		resp, err := SwaggerUIHandler(ctx)
		require.NoError(t, err, "Expected err to be nil")

		fileResponse, ok := resp.(response.File)
		if !ok {
			t.Errorf("Expected a FileResponse")
		}

		if strings.Split(fileResponse.ContentType, ";")[0] != tc.contentType {
			t.Errorf("Expected content type '%s', got '%s'", tc.contentType, fileResponse.ContentType)
		}
	}
}

func TestSwaggerUIHandler_Error(t *testing.T) {
	testContainer, _ := container.NewMockContainer(t)

	testReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/swagger"+"/abc.abc", http.NoBody)
	testReq = mux.SetURLVars(testReq, map[string]string{"name": "abc.abc"})

	gofrReq := gofrHTTP.NewRequest(testReq)
	ctx := newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet), gofrReq, testContainer)
	ctx.ContextLogger = *logging.NewContextLogger(ctx, testContainer.Logger)

	resp, err := SwaggerUIHandler(ctx)

	require.Nil(t, resp)

	var notFound gofrHTTP.ErrorEntityNotFound
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, "file", notFound.Name)
	assert.Equal(t, "abc.abc", notFound.Value)
	assert.Equal(t, http.StatusNotFound, notFound.StatusCode())
}

func TestSwaggerUIHandler_NoFileExtension(t *testing.T) {
	testContainer, _ := container.NewMockContainer(t)

	testReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/swagger/noextension", http.NoBody)
	testReq = mux.SetURLVars(testReq, map[string]string{"name": "noextension"})

	gofrReq := gofrHTTP.NewRequest(testReq)
	ctx := newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet), gofrReq, testContainer)
	ctx.ContextLogger = *logging.NewContextLogger(ctx, testContainer.Logger)

	resp, err := SwaggerUIHandler(ctx)

	assert.Nil(t, resp)

	expectedErr := gofrHTTP.ErrorEntityNotFound{Name: "file", Value: "noextension"}
	assert.Equal(t, expectedErr, err)
}

func TestSwaggerRoutesOnlyOwnEmbeddedAssets(t *testing.T) {
	setupOpenAPIStaticFile(t, []byte(`{"openapi":"3.0.0"}`))

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
	setupOpenAPIStaticFile(t, []byte(`{"openapi":"3.0.0"}`))

	app := New()
	app.httpRegistered = true
	app.checkAndAddOpenAPIDocumentation()

	app.GET("/.well-known/security.txt", func(*Context) (any, error) {
		return "application-defined-security-file", nil
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/security.txt", http.NoBody)
	app.httpServer.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "application-defined-security-file")
}
