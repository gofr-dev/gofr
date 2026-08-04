package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestRouter(t *testing.T) {
	port := testutil.GetFreePort(t)

	cfg := map[string]string{"HTTP_PORT": fmt.Sprint(port), "LOG_LEVEL": "INFO"}
	c := container.NewContainer(config.NewMockConfig(cfg))

	c.Metrics().NewCounter("test-counter", "test")

	// Create a new router instance using the mock container
	router := NewRouter()

	// Add a test handler to the router
	router.Add(http.MethodGet, "/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send a request to the test handler
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRouterWithMiddleware(t *testing.T) {
	port := testutil.GetFreePort(t)

	cfg := map[string]string{"HTTP_PORT": fmt.Sprint(port), "LOG_LEVEL": "INFO"}
	c := container.NewContainer(config.NewMockConfig(cfg))

	c.Metrics().NewCounter("test-counter", "test")

	// Create a new router instance using the mock container
	router := NewRouter()

	router.UseMiddleware(func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-Middleware", "applied")
			inner.ServeHTTP(w, r)
		})
	})

	// Add a test handler to the router
	router.Add(http.MethodGet, "/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send a request to the test handler
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, rec.Code)
	// checking if the testMiddleware has added the required header in the response properly.
	testHeaderValue := rec.Header().Get("X-Test-Middleware")
	assert.Equal(t, "applied", testHeaderValue, "Test_UseMiddleware Failed! header value mismatch.")
}

// TestRouter_DoubleSlashPath_GET verifies that GET requests with double slashes
// are normalized and routed correctly to the GET handler.
func TestRouter_DoubleSlashPath_GET(t *testing.T) {
	router := NewRouter()

	getHandlerCalled := false
	postHandlerCalled := false

	// Register both GET and POST handlers for /hello
	router.Add(http.MethodGet, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		getHandlerCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("GET handler"))
	}))

	router.Add(http.MethodPost, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		postHandlerCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("POST handler"))
	}))

	tests := []struct {
		desc string
		path string
	}{
		{desc: "GET request to /hello", path: "/hello"},
		{desc: "GET request to //hello", path: "//hello"},
		{desc: "GET request to ///hello", path: "///hello"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			getHandlerCalled = false
			postHandlerCalled = false

			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "Status code mismatch")
			assert.True(t, getHandlerCalled, "GET handler should be called")
			assert.False(t, postHandlerCalled, "POST handler should NOT be called")
			assert.Equal(t, "GET handler", rec.Body.String(), "Response body mismatch")
			assert.Empty(t, rec.Header().Get("Location"), "No redirect should be issued")
		})
	}
}

// TestIsCleanPath pins the fast-path predicate that ServeHTTP uses to skip
// path.Clean for already-canonical URLs. A wrong negative would re-run the
// normalizer pointlessly (performance regression); a wrong positive would
// route a non-canonical URL without cleaning it (correctness regression).
func TestIsCleanPath(t *testing.T) {
	canonical := []string{
		"/",
		"/users",
		"/api/v1/things",
		"/users/42",
		"/a/b/c/d",
		"/path-with-dashes",
		"/path.with.dots",
		"/users/42.json",
	}
	dirty := []string{
		"",             // empty
		"users",        // no leading slash
		"//",           // double slash root
		"//users",      // leading double slash
		"/users//42",   // mid double slash
		"/.",           // trailing /.
		"/..",          // trailing /..
		"/./users",     // /./
		"/../users",    // /../
		"/users/.",     // /.
		"/users/..",    // /..
		"/users/./42",  // /./ mid
		"/users/../42", // /../ mid
		"/users/",      // trailing slash on non-root
	}

	for _, p := range canonical {
		if !isCleanPath(p) {
			t.Errorf("isCleanPath(%q) = false, want true", p)
		}
	}

	for _, p := range dirty {
		if isCleanPath(p) {
			t.Errorf("isCleanPath(%q) = true, want false", p)
		}
	}
}

// TestRouter_PathNormalization tests the path normalization function directly.
func TestRouter_PathNormalization(t *testing.T) {
	router := NewRouter()

	// Register handlers for testing
	router.Add(http.MethodGet, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))

	router.Add(http.MethodGet, "/api/v1/users", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("users"))
	}))

	router.Add(http.MethodGet, "/bar", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("bar"))
	}))

	tests := []struct {
		name         string
		input        string
		expectedCode int
		expectedBody string
	}{
		{name: "simple path", input: "/hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "double slash", input: "//hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "triple slash", input: "///hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "multiple slashes in middle", input: "/api//v1///users", expectedCode: http.StatusOK, expectedBody: "users"},
		{name: "current directory dot", input: "/.", expectedCode: http.StatusNotFound, expectedBody: "404 page not found\n"},
		{name: "parent directory", input: "/..", expectedCode: http.StatusNotFound, expectedBody: "404 page not found\n"},
		{name: "relative path no leading slash", input: "/hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "parent directory traversal", input: "/foo/../bar", expectedCode: http.StatusOK, expectedBody: "bar"},
		{name: "parent directory with relative path", input: "/../hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "root path", input: "/", expectedCode: http.StatusNotFound, expectedBody: "404 page not found\n"},
		{name: "empty path", input: "/", expectedCode: http.StatusNotFound, expectedBody: "404 page not found\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.input, http.NoBody)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedCode, rec.Code, "Status code mismatch")
			assert.Equal(t, tc.expectedBody, rec.Body.String(), "Response body mismatch")
		})
	}
}

// TestRouter_DoubleSlashPath_POST verifies that POST requests with double slashes
// are normalized and routed correctly to the POST handler.
func TestRouter_DoubleSlashPath_POST(t *testing.T) {
	router := NewRouter()

	getHandlerCalled := false
	postHandlerCalled := false

	// Register both GET and POST handlers for /hello
	router.Add(http.MethodGet, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		getHandlerCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("GET handler"))
	}))

	router.Add(http.MethodPost, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		postHandlerCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("POST handler"))
	}))

	tests := []struct {
		desc string
		path string
	}{
		{desc: "POST request to /hello", path: "/hello"},
		{desc: "POST request to //hello", path: "//hello"},
		{desc: "POST request to ////hello", path: "////hello"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			getHandlerCalled = false
			postHandlerCalled = false

			req := httptest.NewRequest(http.MethodPost, tc.path, http.NoBody)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "Status code mismatch")
			assert.True(t, postHandlerCalled, "POST handler should be called")
			assert.False(t, getHandlerCalled, "GET handler should NOT be called")
			assert.Equal(t, "POST handler", rec.Body.String(), "Response body mismatch")
			assert.Empty(t, rec.Header().Get("Location"), "No redirect should be issued")
		})
	}
}

func Test_StaticFileServing_Static(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name             string
		setupFiles       func() error
		path             string
		staticServerPath string
		expectedCode     int
		expectedBody     string
	}{
		{
			name: "Serve existing file from /static",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, World!"), 0600)
			},
			path:             "/static/test.txt",
			staticServerPath: "/static",
			expectedCode:     http.StatusOK,
			expectedBody:     "Hello, World!",
		},
		{
			name: "Serve existing file from /",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, Root!"), 0600)
			},
			path:             "/test.txt",
			staticServerPath: "/",
			expectedCode:     http.StatusOK,
			expectedBody:     "Hello, Root!",
		},
		{
			name: "Serve existing file from /public",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, Public!"), 0600)
			},
			path:             "/public/test.txt",
			staticServerPath: "/public",
			expectedCode:     http.StatusOK,
			expectedBody:     "Hello, Public!",
		},
		{
			name: "Serve 404.html for non-existent file",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "404.html"), []byte("<html>404 Not Found</html>"), 0600)
			},
			path:             "/static/nonexistent.html",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "<html>404 Not Found</html>",
		},
		{
			name: "Serve default 404 message when 404.html is missing",
			setupFiles: func() error {
				return os.Remove(filepath.Join(tempDir, "404.html"))
			},
			path:             "/static/nonexistent.html",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "404 Not Found",
		},
		{
			name: "Access forbidden OpenAPI JSON",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, DefaultSwaggerFileName), []byte(`{"openapi": "3.0.0"}`), 0600)
			},
			path:             "/static/openapi.json",
			staticServerPath: "/static",
			expectedCode:     http.StatusForbidden,
			expectedBody:     "403 Forbidden",
		},
		{
			// The name is matched case-insensitively: on a case-insensitive filesystem this URL
			// opens the very file the exact comparison refused. The restriction is applied before
			// the file is resolved, so the case that is served is not what decides it and this
			// holds on a case-sensitive filesystem too.
			name: "Access forbidden OpenAPI JSON in a different case",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, DefaultSwaggerFileName), []byte(`{"openapi": "3.0.0"}`), 0600)
			},
			path:             "/static/openapi.JSON",
			staticServerPath: "/static",
			expectedCode:     http.StatusForbidden,
			expectedBody:     "403 Forbidden",
		},
		{
			// An unreadable file is the caller being refused, not the server failing — the mapping
			// net/http's own toHTTPError makes. This asserted 500 before, which tells a monitor the
			// application is broken when what it has is a file mode.
			name: "Serving File with no Read permission",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "restricted.txt"), []byte("Restricted content"), 0000)
			},
			path:             "/static/restricted.txt",
			staticServerPath: "/static",
			expectedCode:     http.StatusForbidden,
			expectedBody:     "403 Forbidden",
		},
	}

	runStaticFileTests(t, tempDir, testCases)
}

// Test_StaticFileServing_EndpointRoot covers requests for the root of a static endpoint, as opposed
// to a file beneath it. The root resolves to the served directory itself, which both the
// containment check and the routing have to admit without also admitting a sibling endpoint.
func Test_StaticFileServing_EndpointRoot(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name             string
		setupFiles       func() error
		path             string
		staticServerPath string
		expectedCode     int
		expectedBody     string
	}{
		{
			// The endpoint root resolves to the served directory itself, which the containment
			// check must admit — see Test_isRestrictedFile.
			name: "Serve index.html at the root endpoint's root",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html>Index</html>"), 0600)
			},
			path:             "/",
			staticServerPath: "/",
			expectedCode:     http.StatusOK,
			expectedBody:     "<html>Index</html>",
		},
		{
			// Router.ServeHTTP cleans the trailing slash off "/static/", so the endpoint root
			// arrives as "/static" and must be routed by something.
			name: "Serve index.html at a non-root endpoint's root, with trailing slash",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html>Index</html>"), 0600)
			},
			path:             "/static/",
			staticServerPath: "/static",
			expectedCode:     http.StatusOK,
			expectedBody:     "<html>Index</html>",
		},
		{
			name: "Serve index.html at a non-root endpoint's root, without trailing slash",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html>Index</html>"), 0600)
			},
			path:             "/static",
			staticServerPath: "/static",
			expectedCode:     http.StatusOK,
			expectedBody:     "<html>Index</html>",
		},
		{
			// The prefix route must keep its trailing separator: a sibling endpoint sharing the
			// prefix is not this endpoint and must not be served from its directory.
			name: "Sibling endpoint sharing the prefix is not served",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html>Index</html>"), 0600)
			},
			path:             "/staticother/index.html",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "404 page not found",
		},
	}

	runStaticFileTests(t, tempDir, testCases)
}

// Test_StaticFileServing_EndpointRootWithoutServableIndex covers an endpoint root that resolves to
// a directory holding no file the handler can serve. None of these may be answered with a listing
// of the directory's contents, and none may reach http.ServeContent: with http.FileServer gone,
// validateFile's file type check is the only thing standing between a non-regular path and a 200
// whose body never arrives.
func Test_StaticFileServing_EndpointRootWithoutServableIndex(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name             string
		setupFiles       func() error
		path             string
		staticServerPath string
		expectedCode     int
		expectedBody     string
	}{
		{
			// Without an index file the directory must not be handed to http.FileServer, which
			// would answer with a listing of its contents.
			name: "Endpoint root without an index file is not listed",
			setupFiles: func() error {
				return os.RemoveAll(filepath.Join(tempDir, "index.html"))
			},
			path:             "/static",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "404 Not Found",
		},
		{
			// Nothing checks the type of the resolved path once http.FileServer is gone, so a
			// directory named index.html would reach http.ServeContent — which writes a
			// Content-Length from the directory's FileInfo and then no body, a 200 the client
			// reads as an unexpected EOF. It has to take the not-found path instead.
			name: "Endpoint root whose index.html is a directory is not served",
			setupFiles: func() error {
				return os.MkdirAll(filepath.Join(tempDir, "index.html"), 0750)
			},
			path:             "/static",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "404 Not Found",
		},
		{
			// A subdirectory root resolves to a directory the same way the endpoint root does, and
			// is likewise answered rather than listed when it holds no index file.
			name: "Subdirectory without an index file is not listed",
			setupFiles: func() error {
				return os.MkdirAll(filepath.Join(tempDir, "sub"), 0750)
			},
			path:             "/static/sub",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "404 Not Found",
		},
		{
			// A root without an index file takes the ordinary not-found path, so the directory's own
			// 404.html answers it — the same page a missing file gets, not a mux default.
			name: "Endpoint root without an index file serves 404.html",
			setupFiles: func() error {
				if err := os.RemoveAll(filepath.Join(tempDir, "index.html")); err != nil {
					return err
				}

				return os.WriteFile(filepath.Join(tempDir, "404.html"), []byte("<html>404 Not Found</html>"), 0600)
			},
			path:             "/static",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "<html>404 Not Found</html>",
		},
	}

	runStaticFileTests(t, tempDir, testCases)
}

// Test_StaticFileServing_DirectoryNameForms covers the forms a caller can give the served
// directory in. staticHandler builds the requested path with filepath.Abs, so the containment
// check compares an absolute, cleaned path against directoryName as it was handed in: a relative
// name, or an absolute one carrying a trailing separator, never matches and every request under
// the endpoint — root or not — is answered with 403. App.AddStaticFiles resolves "./x" and "x"
// against the working directory but leaves an absolute name untouched, so the trailing-separator
// form reaches here from the public API.
func Test_StaticFileServing_DirectoryNameForms(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html>Index</html>"), 0600))

	workingDir, err := os.Getwd()
	require.NoError(t, err)

	relativeDir, err := filepath.Rel(workingDir, tempDir)
	require.NoError(t, err)

	sep := string(os.PathSeparator)

	tests := []struct {
		name    string
		dirName string
	}{
		{"absolute", tempDir},
		{"absolute with trailing separator", tempDir + sep},
		{"absolute unclean", tempDir + sep + "." + sep},
		{"relative", relativeDir},
		{"relative with dot prefix", "." + sep + relativeDir},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{"/static", "/static/index.html"} {
				router := NewRouter()
				router.AddStaticFiles(logging.NewMockLogger(logging.DEBUG), "/static", tc.dirName)

				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody))

				assert.Equal(t, http.StatusOK, w.Code, "GET %s", path)
				assert.Equal(t, "<html>Index</html>", strings.TrimSpace(w.Body.String()), "GET %s", path)
			}
		})
	}
}

// Test_StaticFileServing_MethodNotAllowed covers the methods a static endpoint does not implement.
// The routes carry no .Methods() matcher — deliberately, because restricting them there drops the
// request to the catch-all and answers 404, which contradicts the 200 the same path gives for GET —
// so the handler is the only thing standing between a DELETE and a 200 carrying the file's content.
func Test_StaticFileServing_MethodNotAllowed(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html>Index</html>"), 0600))

	tests := []struct {
		name          string
		method        string
		path          string
		expectedCode  int
		expectedBody  string
		expectedAllow string
	}{
		{"GET at the endpoint root is served", http.MethodGet, "/static", http.StatusOK, "<html>Index</html>", ""},
		{"GET of a file is served", http.MethodGet, "/static/index.html", http.StatusOK, "<html>Index</html>", ""},
		// HEAD is a GET without the body: http.ServeContent writes the headers and omits it.
		{"HEAD at the endpoint root is served", http.MethodHead, "/static", http.StatusOK, "", ""},
		{"POST is refused", http.MethodPost, "/static", http.StatusMethodNotAllowed, "405 Method Not Allowed", "GET, HEAD"},
		{"PUT is refused", http.MethodPut, "/static/index.html", http.StatusMethodNotAllowed, "405 Method Not Allowed", "GET, HEAD"},
		{"PATCH is refused", http.MethodPatch, "/static", http.StatusMethodNotAllowed, "405 Method Not Allowed", "GET, HEAD"},
		// The one that reads as an outright lie without this: "200, deleted, here it is."
		{"DELETE is refused", http.MethodDelete, "/static", http.StatusMethodNotAllowed, "405 Method Not Allowed", "GET, HEAD"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouter()
			router.AddStaticFiles(logging.NewMockLogger(logging.DEBUG), "/static", tempDir)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, http.NoBody))

			assert.Equal(t, tc.expectedCode, w.Code)
			assert.Equal(t, tc.expectedBody, strings.TrimSpace(w.Body.String()))

			// RFC 9110 §15.5.6 requires Allow on a 405, and requires it absent nowhere else here.
			assert.Equal(t, tc.expectedAllow, w.Header().Get("Allow"))
		})
	}
}

// Test_StaticFileServing_PermissionDenied covers a path the process is not allowed to read. It is
// the caller being refused rather than the server failing, which is the mapping net/http's own
// toHTTPError makes and the one nginx and Apache make; 500 would tell a monitor the application is
// broken when what it has is a file mode. Two routes reach it — validateFile's own
// errReadPermissionDenied, and a real EACCES from os.Stat — and they must not answer differently.
func Test_StaticFileServing_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: the permission bits under test are not enforced")
	}

	tempDir := t.TempDir()

	testCases := []struct {
		name             string
		setupFiles       func() error
		path             string
		staticServerPath string
		expectedCode     int
		expectedBody     string
	}{
		{
			// Caught by validateFile's mode check, which returns errReadPermissionDenied — an error
			// that has to satisfy fs.ErrPermission to be mapped rather than fall through to 500.
			name: "Unreadable file is forbidden, not a server error",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "noread.txt"), []byte("secret"), 0000)
			},
			path:             "/static/noread.txt",
			staticServerPath: "/static",
			expectedCode:     http.StatusForbidden,
			expectedBody:     "403 Forbidden",
		},
		{
			// The other route: the mode check never runs, because os.Stat of the index file inside an
			// untraversable directory fails with a real EACCES first.
			name: "Untraversable directory is forbidden, not a server error",
			setupFiles: func() error {
				nodir := filepath.Join(tempDir, "nodir")
				if err := os.MkdirAll(nodir, 0750); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(nodir, "index.html"), []byte("<html>X</html>"), 0600); err != nil {
					return err
				}

				return os.Chmod(nodir, 0000)
			},
			path:             "/static/nodir",
			staticServerPath: "/static",
			expectedCode:     http.StatusForbidden,
			expectedBody:     "403 Forbidden",
		},
	}

	// t.TempDir's cleanup cannot walk a directory it has no permission to enter.
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(tempDir, "nodir"), 0750)
	})

	runStaticFileTests(t, tempDir, testCases)
}

// Test_StaticFileServing_NonDirectoryDirName covers a served path that is not a directory.
// App.AddStaticFiles only stats it, so a regular file passes registration, and the containment
// check admits the served path itself now that the endpoint root has to resolve to it — together
// that would serve the file verbatim at the endpoint. The old prefix-only comparison failed closed
// here by accident, and registration has to keep it closed on purpose.
func Test_StaticFileServing_NonDirectoryDirName(t *testing.T) {
	tempDir := t.TempDir()

	configFile := filepath.Join(tempDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("db_password: hunter2"), 0600))

	tests := []struct {
		name    string
		dirName string
	}{
		{"regular file", configFile},
		{"path that does not exist", filepath.Join(tempDir, "absent")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouter()
			router.AddStaticFiles(logging.NewMockLogger(logging.DEBUG), "/data", tc.dirName)

			for _, path := range []string{"/data", "/data/"} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody))

				// Nothing was registered, so the request is unrouted rather than refused by the handler.
				assert.Equal(t, http.StatusNotFound, w.Code, "GET %s", path)
				assert.NotContains(t, w.Body.String(), "hunter2", "GET %s", path)
			}
		})
	}
}

// Test_StaticFileServing_NoRedirect covers the paths a file server answers with a redirect rather
// than content. Every such redirect points at a trailing-slash or "./" form of the URL, and
// Router.ServeHTTP's path.Clean strips that form straight back off, so the client is sent in a
// circle it can never satisfy. runStaticFileTests asserts no Location header on every case; these
// are the requests that produce one if the serving path ever goes back through http.FileServer or
// http.ServeFile.
func Test_StaticFileServing_NoRedirect(t *testing.T) {
	tempDir := t.TempDir()

	setupSub := func() error {
		if err := os.MkdirAll(filepath.Join(tempDir, "sub"), 0750); err != nil {
			return err
		}

		return os.WriteFile(filepath.Join(tempDir, "sub", "index.html"), []byte("<html>Sub</html>"), 0600)
	}

	testCases := []struct {
		name             string
		setupFiles       func() error
		path             string
		staticServerPath string
		expectedCode     int
		expectedBody     string
	}{
		{
			// http.FileServer answers a directory with "301 Location: sub/".
			name:             "Subdirectory is served through its index file",
			setupFiles:       setupSub,
			path:             "/static/sub",
			staticServerPath: "/static",
			expectedCode:     http.StatusOK,
			expectedBody:     "<html>Sub</html>",
		},
		{
			// The endpoint's own index file. After StripPrefix the request path is "index.html",
			// which misses http.ServeFile's "/index.html" suffix rule — but http.FileServer, which
			// sees the unstripped path, answered this with "301 Location: ./".
			name: "Explicit index.html at the endpoint root",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html>Index</html>"), 0600)
			},
			path:             "/static/index.html",
			staticServerPath: "/static",
			expectedCode:     http.StatusOK,
			expectedBody:     "<html>Index</html>",
		},
		{
			// The one path that trips http.ServeFile's rule as well: after StripPrefix the request
			// path is "sub/index.html", which does end in "/index.html". http.ServeContent, which
			// is handed an open file rather than a URL, has no such rule.
			name:             "Explicit index.html in a subdirectory",
			setupFiles:       setupSub,
			path:             "/static/sub/index.html",
			staticServerPath: "/static",
			expectedCode:     http.StatusOK,
			expectedBody:     "<html>Sub</html>",
		},
		{
			// A miss whose own path ends in "/index.html" must still get the 404 page, not a
			// redirect issued on the way to serving it.
			name: "Missing index.html in a subdirectory serves 404.html",
			setupFiles: func() error {
				if err := os.MkdirAll(filepath.Join(tempDir, "empty"), 0750); err != nil {
					return err
				}

				return os.WriteFile(filepath.Join(tempDir, "404.html"), []byte("<html>404 Not Found</html>"), 0600)
			},
			path:             "/static/empty/index.html",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "<html>404 Not Found</html>",
		},
	}

	runStaticFileTests(t, tempDir, testCases)
}

func Test_isRestrictedFile(t *testing.T) {
	tests := []struct {
		name          string
		directoryName string
		url           string
		absPath       string
		expected      bool
	}{
		{
			name:          "file inside static directory is not restricted",
			directoryName: "/app/public",
			url:           "/index.html",
			absPath:       "/app/public/index.html",
			expected:      false,
		},
		{
			name:          "openapi.json inside static directory is restricted",
			directoryName: "/app/public",
			url:           "/openapi.json",
			absPath:       "/app/public/openapi.json",
			expected:      true,
		},
		{
			// A case-insensitive filesystem opens the same spec for this name, so the restriction
			// has to cover every spelling of it rather than the one canonical form.
			name:          "openapi.json in a different case is restricted",
			directoryName: "/app/public",
			url:           "/openapi.JSON",
			absPath:       "/app/public/openapi.JSON",
			expected:      true,
		},
		{
			name:          "file outside static directory is restricted",
			directoryName: "/app/public",
			url:           "/secret.txt",
			absPath:       "/app/secret.txt",
			expected:      true,
		},
		{
			name:          "sibling directory with shared prefix is restricted",
			directoryName: "/app/public",
			url:           "/secret.txt",
			absPath:       "/app/publicother/secret.txt",
			expected:      true,
		},
		{
			name:          "nested file inside static directory is not restricted",
			directoryName: "/app/public",
			url:           "/sub/page.html",
			absPath:       "/app/public/sub/page.html",
			expected:      false,
		},
		{
			// A request for the endpoint root resolves to the directory itself, with no trailing
			// separator. It is the directory being served, not an escape from it.
			name:          "static directory itself is not restricted",
			directoryName: "/app/public",
			url:           "/",
			absPath:       "/app/public",
			expected:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := staticFileConfig{directoryName: tc.directoryName}
			result := cfg.isRestrictedFile(tc.url, tc.absPath)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func runStaticFileTests(t *testing.T, tempDir string, testCases []struct {
	name             string
	setupFiles       func() error
	path             string
	staticServerPath string
	expectedCode     int
	expectedBody     string
}) {
	t.Helper()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.setupFiles(); err != nil {
				t.Fatalf("Failed to set up files: %v", err)
			}

			logger := logging.NewMockLogger(logging.DEBUG)

			router := NewRouter()
			router.AddStaticFiles(logger, tc.staticServerPath, tempDir)

			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code)
			assert.Equal(t, tc.expectedBody, strings.TrimSpace(w.Body.String()))

			// A static endpoint never redirects. Every redirect the file server used to issue
			// pointed at a trailing-slash or "./" form that Router.ServeHTTP's path.Clean undoes,
			// so any Location here is a client loop waiting to happen.
			assert.Empty(t, w.Header().Get("Location"), "static file serving must not redirect")
		})
	}
}

// discardingResponseWriter is a zero-cost ResponseWriter for benchmarks
// that should measure routing/handler cost without including the per-iter
// allocation of httptest.NewRecorder. Shared across the http-package
// benchmarks (router, responder).
type discardingResponseWriter struct {
	h http.Header
}

func (d *discardingResponseWriter) Header() http.Header {
	if d.h == nil {
		d.h = http.Header{}
	}

	return d.h
}

func (*discardingResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (*discardingResponseWriter) WriteHeader(int)             {}

// noopHandler returns an http.Handler that discards the request and
// writes nothing. Used by router benchmarks to isolate router cost from
// any handler-side work.
func noopHandler() http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
}

// BenchmarkRouter_Get_Static measures the per-request cost of routing
// a request to an exact-match route (no path parameters). Closest
// approximation to TFB's /plaintext routing cost.
//
// Hot path under measurement:
//   - Router.ServeHTTP path normalization (ServeHTTP, this file)
//   - gorilla/mux.Router.ServeHTTP route matching
//
// PR targets that should move this number:
//   - PR-12 (path-clean fast path) — small win
//   - PR-N (gorilla/mux → chi) — largest win, separate initiative
func BenchmarkRouter_Get_Static(b *testing.B) {
	r := NewRouter()
	r.Add(http.MethodGet, "/plaintext", noopHandler())

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/plaintext", http.NoBody)
	w := &discardingResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkRouter_Get_PathParam measures the per-request cost of routing
// to a path with a parameter ({id}). Includes gorilla/mux's parameter
// extraction overhead, which is part of why mux is slower than radix-tree
// routers (chi, httprouter).
func BenchmarkRouter_Get_PathParam(b *testing.B) {
	r := NewRouter()
	r.Add(http.MethodGet, "/users/{id}", noopHandler())

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/users/42", http.NoBody)
	w := &discardingResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}
