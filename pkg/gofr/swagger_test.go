package gofr

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
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
	if err := ioutil.WriteFile(openAPIFilePath, openAPIContent, 0644); err != nil {
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

	if string(fileResponse.Content) != string(openAPIContent) {
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
