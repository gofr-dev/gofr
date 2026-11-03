package gofr

import (
    "bytes"
    "context"
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

func TestOpenAPIHandler(t *testing.T) {
    t.Chdir(t.TempDir())

    openAPIFilePath := filepath.Join("static", OpenAPIJSON)
    require.NoError(t, os.MkdirAll(filepath.Dir(openAPIFilePath), 0700))

    openAPIContent := []byte(`{"swagger": "2.0", "info": {"version": "1.0.0", "title": "Sample API"}}`)
    require.NoError(t, os.WriteFile(openAPIFilePath, openAPIContent, 0600))

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

    assert.Nil(t, result, "Expected result to be nil")
    require.ErrorIs(t, err, os.ErrNotExist)
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

    assert.Nil(t, resp)
    require.ErrorIs(t, err, os.ErrNotExist)
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
