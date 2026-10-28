package gofr

import (
	"embed"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
)

//go:embed static/*
var fs embed.FS

const (
	OpenAPIJSON = "openapi.json"

	// swaggerFileNameParam is used by direct callers of SwaggerUIHandler.
	swaggerFileNameParam = "name"
)

// OpenAPIHandler serves the `openapi.json` file at the specified path.
// It reads the file from the disk and returns its content as a response.
func OpenAPIHandler(c *Context) (any, error) {
	rootDir, _ := os.Getwd()
	filePath := filepath.Join(rootDir, "static", OpenAPIJSON)

	b, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		c.Errorf("Failed to read OpenAPI JSON file at path %s: %v", filePath, err)
		return nil, err
	}

	return response.File{Content: b, ContentType: contentTypeJSON}, nil
}

// SwaggerUIHandler serves the Swagger UI index. It also supports an explicit
// filename path parameter for callers that invoke the handler directly.
func SwaggerUIHandler(c *Context) (any, error) {
	fileName := c.PathParam(swaggerFileNameParam)
	if fileName == "" {
		fileName = "index.html"
	}

	return serveSwaggerUIAsset(c, fileName)
}

// serveSwaggerUIAsset is shared by the Swagger index route and the individual
// routes registered for embedded UI assets.
func serveSwaggerUIAsset(c *Context, fileName string) (any, error) {
	ext := filepath.Ext(fileName)
	if ext == "" {
		return nil, gofrHTTP.ErrorEntityNotFound{Name: "file", Value: fileName}
	}

	data, err := fs.ReadFile("static/" + fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, gofrHTTP.ErrorEntityNotFound{Name: "file", Value: fileName}
		}

		c.Errorf("Failed to read Swagger UI file %s from embedded file system: %v", fileName, err)
		return nil, err
	}

	return response.File{Content: data, ContentType: mime.TypeByExtension(ext)}, nil
}

// isSwaggerUIAsset limits the public routes to web assets, excluding embedded
// implementation files such as static/files.go.
func isSwaggerUIAsset(name string) bool {
	switch filepath.Ext(name) {
	case ".css", ".html", ".ico", ".js", ".png":
		return true
	default:
		return false
	}
}

func (a *App) checkAndAddOpenAPIDocumentation() {
	// Only expose Swagger documentation when the application's OpenAPI file exists.
	if _, err := os.Stat("./static/" + gofrHTTP.DefaultSwaggerFileName); err != nil {
		return
	}

	a.add(http.MethodGet, "/.well-known/"+gofrHTTP.DefaultSwaggerFileName, OpenAPIHandler)
	a.add(http.MethodGet, "/.well-known/swagger", SwaggerUIHandler)

	// Enumerate the embedded assets at startup instead of claiming every
	// /.well-known/{name} path. Other well-known resources remain routable by
	// the application (or fall through to its ordinary 404 handler).
	entries, err := fs.ReadDir("static")
	if err != nil {
		a.container.Errorf("Failed to list embedded Swagger UI assets: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !isSwaggerUIAsset(entry.Name()) {
			continue
		}

		fileName := entry.Name()
		a.add(http.MethodGet, "/.well-known/"+fileName, func(c *Context) (any, error) {
			return serveSwaggerUIAsset(c, fileName)
		})
	}
}
