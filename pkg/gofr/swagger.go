package gofr

import (
	"embed"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
)

//go:embed static/*
var fs embed.FS

const (
	OpenAPIJSON = "openapi.json"
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

	return response.File{Content: b, ContentType: "application/json"}, nil
}

// SwaggerUIHandler serves the static files of the Swagger UI.
func SwaggerUIHandler(c *Context) (any, error) {
	fileName := c.PathParam("name")
	if fileName == "" {
		// Read the index.html file
		fileName = "index.html"
	}

	data, err := fs.ReadFile("static/" + fileName)
	if err != nil {
		c.Errorf("Failed to read Swagger UI file %s from embedded file system: %v", fileName, err)
		return nil, err
	}

	split := strings.Split(fileName, ".")

	ct := mime.TypeByExtension("." + split[1])

	// Return the rendered HTML as a string
	return response.File{Content: data, ContentType: ct}, nil
}

func (a *App) checkAndAddOpenAPIDocumentation() {
	// If the openapi.json file exists in the static directory, set up routes for OpenAPI and Swagger documentation.
	if _, err := os.Stat("./static/" + gofrHTTP.DefaultSwaggerFileName); err == nil {
		// Route to serve the OpenAPI JSON specification file.
		a.add(http.MethodGet, "/.well-known/"+gofrHTTP.DefaultSwaggerFileName, OpenAPIHandler)
		// Route to serve the Swagger UI, providing a user interface for the API documentation.
		a.add(http.MethodGet, "/.well-known/swagger", SwaggerUIHandler)
		// Catchall route: any request to /.well-known/{name} (e.g., /.well-known/other)
		// will be handled by the SwaggerUIHandler, serving the Swagger UI.
		a.add(http.MethodGet, "/.well-known/{name}", SwaggerUIHandler)
	}
}
