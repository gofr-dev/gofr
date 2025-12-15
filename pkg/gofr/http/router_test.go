package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gofr.dev/pkg/gofr/logging"
)

func TestConstants(t *testing.T) {
	if DefaultSwaggerFileName != "openapi.json" {
		t.Errorf("Expected DefaultSwaggerFileName to be 'openapi.json', got %s", DefaultSwaggerFileName)
	}
	if staticServerNotFoundFileName != "404.html" {
		t.Errorf("Expected staticServerNotFoundFileName to be '404.html', got %s", staticServerNotFoundFileName)
	}
	if errReadPermissionDenied.Error() != "file does not have read permission" {
		t.Errorf("Expected error message 'file does not have read permission', got %s", errReadPermissionDenied.Error())
	}
}

func TestNewRouter(t *testing.T) {
	router := NewRouter()
	if router == nil {
		t.Error("NewRouter should return a non-nil router")
	}
	if router.RegisteredRoutes == nil {
		t.Error("RegisteredRoutes should be initialized")
	}
}

func TestRouterAdd(t *testing.T) {
	router := NewRouter()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})

	router.Add("GET", "/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestUseMiddleware(t *testing.T) {
	router := NewRouter()
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "middleware")
			next.ServeHTTP(w, r)
		})
	}

	router.UseMiddleware(middleware)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})
	router.Add("GET", "/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("X-Test") != "middleware" {
		t.Error("Middleware was not applied")
	}
}

func TestAddStaticFiles(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	logger := &mockLogger{}
	router := NewRouter()
	router.AddStaticFiles(logger, "/static", tempDir)

	req := httptest.NewRequest("GET", "/static/test.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test content") {
		t.Error("Expected file content to be served")
	}
}

func TestAddStaticFilesRootEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "index.html")
	os.WriteFile(testFile, []byte("<html></html>"), 0644)

	logger := &mockLogger{}
	router := NewRouter()
	router.AddStaticFiles(logger, "/", tempDir)

	req := httptest.NewRequest("GET", "/index.html", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestStaticHandlerRestrictedFile(t *testing.T) {
	tempDir := t.TempDir()
	swaggerFile := filepath.Join(tempDir, DefaultSwaggerFileName)
	os.WriteFile(swaggerFile, []byte("swagger content"), 0644)

	logger := &mockLogger{}
	router := NewRouter()
	router.AddStaticFiles(logger, "/static", tempDir)

	req := httptest.NewRequest("GET", "/static/"+DefaultSwaggerFileName, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestStaticHandlerFileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	logger := &mockLogger{}
	router := NewRouter()
	router.AddStaticFiles(logger, "/static", tempDir)

	req := httptest.NewRequest("GET", "/static/nonexistent.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

func TestStaticHandlerCustom404(t *testing.T) {
	tempDir := t.TempDir()
	notFoundFile := filepath.Join(tempDir, staticServerNotFoundFileName)
	os.WriteFile(notFoundFile, []byte("Custom 404"), 0644)

	logger := &mockLogger{}
	router := NewRouter()
	router.AddStaticFiles(logger, "/static", tempDir)

	req := httptest.NewRequest("GET", "/static/nonexistent.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Custom 404") {
		t.Error("Expected custom 404 content")
	}
}

func TestStaticHandlerNoReadPermission(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "noread.txt")
	os.WriteFile(testFile, []byte("content"), 0000)

	logger := &mockLogger{}
	router := NewRouter()
	router.AddStaticFiles(logger, "/static", tempDir)

	req := httptest.NewRequest("GET", "/static/noread.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500, got %d", w.Code)
	}
}

func TestIsRestrictedFile(t *testing.T) {
	cfg := staticFileConfig{directoryName: "/test"}

	tests := []struct {
		url     string
		absPath string
		want    bool
	}{
		{"/openapi.json", "/test/openapi.json", true},
		{"/test.txt", "/test/test.txt", false},
		{"/test.txt", "/other/test.txt", true},
	}

	for _, tt := range tests {
		got := cfg.isRestrictedFile(tt.url, tt.absPath)
		if got != tt.want {
			t.Errorf("isRestrictedFile(%q, %q) = %v, want %v", tt.url, tt.absPath, got, tt.want)
		}
	}
}

func TestValidateFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	cfg := staticFileConfig{}
	err := cfg.validateFile(testFile)
	if err != nil {
		t.Errorf("validateFile should not return error for valid file: %v", err)
	}

	// Test non-existent file
	err = cfg.validateFile("/nonexistent")
	if err == nil {
		t.Error("validateFile should return error for non-existent file")
	}

	// Test file without read permission
	noReadFile := filepath.Join(tempDir, "noread.txt")
	os.WriteFile(noReadFile, []byte("content"), 0000)
	err = cfg.validateFile(noReadFile)
	if err != errReadPermissionDenied {
		t.Errorf("validateFile should return read permission error, got: %v", err)
	}
}

func TestRespondWithError(t *testing.T) {
	logger := &mockLogger{}
	cfg := staticFileConfig{logger: logger}
	w := httptest.NewRecorder()

	cfg.respondWithError(w, "test message", "/test", fmt.Errorf("test error"), http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "400") {
		t.Error("Response should contain status code")
	}

	// Test with nil error
	w2 := httptest.NewRecorder()
	cfg.respondWithError(w2, "test message", "/test", nil, http.StatusNotFound)
	if w2.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w2.Code)
	}
}

func TestRespondWithFileError(t *testing.T) {
	logger := &mockLogger{}
	cfg := staticFileConfig{directoryName: t.TempDir(), logger: logger}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	// Test with os.ErrNotExist
	cfg.respondWithFileError(w, req, "/nonexistent", os.ErrNotExist)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}

	// Test with other error
	w2 := httptest.NewRecorder()
	cfg.respondWithFileError(w2, req, "/test", fmt.Errorf("other error"))
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500, got %d", w2.Code)
	}
}

func TestDoubleSlashRouting(t *testing.T) {
	router := NewRouter()

	getHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("GET"))
	})
	
	postHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("POST"))
	})

	router.Add("GET", "/hello", getHandler)
	router.Add("POST", "/hello", postHandler)
	router.Add("GET", "//hello", getHandler)
	router.Add("POST", "//hello", postHandler)

	// Test POST with double slash - should work directly
	req := httptest.NewRequest("POST", "//hello", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for POST //hello, got %d", w.Code)
	}
	if w.Body.String() != "POST" {
		t.Errorf("Expected POST response, got %s", w.Body.String())
	}

	// Test GET with double slash - should work directly
	req2 := httptest.NewRequest("GET", "//hello", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected 200 for GET //hello, got %d", w2.Code)
	}
	if w2.Body.String() != "GET" {
		t.Errorf("Expected GET response, got %s", w2.Body.String())
	}
}

func TestStaticHandlerAbsPathError(t *testing.T) {
	logger := &mockLogger{}
	router := NewRouter()
	router.AddStaticFiles(logger, "/static", "/nonexistent")

	// Test with path that causes filepath.Abs to fail (using null byte)
	req := httptest.NewRequest("GET", "/static/\x00", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500, got %d", w.Code)
	}
}

func TestStaticHandlerPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	logger := &mockLogger{}
	router := NewRouter()
	router.AddStaticFiles(logger, "/static", tempDir)

	// Test path traversal attempt
	req := httptest.NewRequest("GET", "/static/../../../etc/passwd", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestUseMiddlewareMultiple(t *testing.T) {
	router := NewRouter()
	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-1", "middleware1")
			next.ServeHTTP(w, r)
		})
	}
	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-2", "middleware2")
			next.ServeHTTP(w, r)
		})
	}

	router.UseMiddleware(middleware1, middleware2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})
	router.Add("GET", "/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("X-Test-1") != "middleware1" {
		t.Error("Middleware 1 was not applied")
	}
	if w.Header().Get("X-Test-2") != "middleware2" {
		t.Error("Middleware 2 was not applied")
	}
}

func TestStaticHandlerDirectCall(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "direct.txt")
	os.WriteFile(testFile, []byte("direct content"), 0644)

	logger := &mockLogger{}
	cfg := staticFileConfig{directoryName: tempDir, logger: logger}
	fileServer := http.FileServer(http.Dir(tempDir))
	handler := cfg.staticHandler(fileServer)

	req := httptest.NewRequest("GET", "/direct.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "direct content") {
		t.Error("Expected file content to be served")
	}
}

type mockLogger struct{}

func (m *mockLogger) Debug(args ...interface{})                   {}
func (m *mockLogger) Debugf(format string, args ...interface{})  {}
func (m *mockLogger) Log(args ...interface{})                    {}
func (m *mockLogger) Logf(format string, args ...interface{})    {}
func (m *mockLogger) Info(args ...interface{})                   {}
func (m *mockLogger) Infof(format string, args ...interface{})   {}
func (m *mockLogger) Notice(args ...interface{})                 {}
func (m *mockLogger) Noticef(format string, args ...interface{}) {}
func (m *mockLogger) Warn(args ...interface{})                   {}
func (m *mockLogger) Warnf(format string, args ...interface{})   {}
func (m *mockLogger) Error(args ...interface{})                  {}
func (m *mockLogger) Errorf(format string, args ...interface{})  {}
func (m *mockLogger) Fatal(args ...interface{})                  {}
func (m *mockLogger) Fatalf(format string, args ...interface{})  {}