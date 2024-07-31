package gofr

import (
	"bytes"
	"errors"
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
)

func TestOpenAPIHandler(t *testing.T) {
	// Create the openapi.json file within the static directory
	openAPIFilePath := filepath.Join("static", OpenAPIJSON)

	openAPIContent := []byte(`{"swagger": "2.0", "info": {"version": "1.0.0", "title": "Sample API"}}`)
	if err := os.WriteFile(openAPIFilePath, openAPIContent, 0600); err != nil {
		t.Fatalf("Failed to create openapi.json file: %v", err)
	}

	// Defer removal of the openapi.json file from static directory
	defer func() {
		if err := os.Remove("static/openapi.json"); err != nil {
			t.Errorf("Failed to remove file from static directory: %v", err)
		}
	}()

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
	testContainer, _ := container.NewMockContainer(t)

	ctx := createTestContext(http.MethodGet, "/.well-known/openapi.json", "", nil, testContainer)

	result, err := OpenAPIHandler(ctx)

	assert.Nil(t, result, "Expected result to be nil")
	errors.Is(err, &os.PathError{Path: "/Users/raramuri/Projects/gofr.dev/gofr/pkg/gofr/static/openapi.json"})
	require.Error(t, err, "Expected error")
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
		testReq := httptest.NewRequest(http.MethodGet, "/.well-known/swagger"+"/"+tc.fileName, http.NoBody)
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

	testReq := httptest.NewRequest(http.MethodGet, "/.well-known/swagger"+"/abc.abc", http.NoBody)
	testReq = mux.SetURLVars(testReq, map[string]string{"name": "abc.abc"})

	gofrReq := gofrHTTP.NewRequest(testReq)
	ctx := newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet), gofrReq, testContainer)

	resp, err := SwaggerUIHandler(ctx)

	assert.Nil(t, resp)
	errors.Is(err, &os.PathError{Path: "/Users/raramuri/Projects/gofr.dev/gofr/pkg/gofr/static/abc.abc"})
}
