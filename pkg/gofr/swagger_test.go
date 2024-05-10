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

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
)

func TestOpenAPIHandler(t *testing.T) {
	// Create the api directory within the temporary directory
	if err := os.Mkdir("api", 0755); err != nil {
		t.Fatalf("Failed to create api directory: %v", err)
	}

	// Create the openapi.json file within the api directory
	openAPIFilePath := filepath.Join("api", OpenAPIJSON)

	openAPIContent := []byte(`{"swagger": "2.0", "info": {"version": "1.0.0", "title": "Sample API"}}`)
	if err := os.WriteFile(openAPIFilePath, openAPIContent, 0600); err != nil {
		t.Fatalf("Failed to create openapi.json file: %v", err)
	}

	// Defer removal of the api directory
	defer func() {
		if err := os.RemoveAll("api"); err != nil {
			t.Fatalf("Failed to remove api directory: %v", err)
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
		t.Fatal("Expected a FileResponse")
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
	errors.Is(err, &os.PathError{Path: "/Users/raramuri/Projects/gofr.dev/gofr/pkg/gofr/api/openapi.json"})
	assert.NotNil(t, err, "Expected error")
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
		assert.Nil(t, err, "Expected err to be nil")

		fileResponse, ok := resp.(response.File)
		if !ok {
			t.Fatal("Expected a FileResponse")
		}

		if strings.Split(fileResponse.ContentType, ";")[0] != tc.contentType {
			t.Errorf("Expected content type 'application/json', got '%s'", fileResponse.ContentType)
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
	errors.Is(err, &os.PathError{Path: "/Users/raramuri/Projects/gofr.dev/gofr/pkg/gofr/swagger/abc.abc"})
}
