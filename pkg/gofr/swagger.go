package gofr

import (
	"embed"
	"mime"
	"os"
	"path/filepath"
	"strings"

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
