package gofr

import (
	"embed"
	"html/template"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"gofr.dev/pkg/gofr/http/response"
)

//go:embed swagger/*
var fs embed.FS

func OpenAPIHandler(c *Context) (interface{}, error) {
	rootDir, _ := os.Getwd()
	fileDir := rootDir + "/" + "api"

	_, err := template.New("openapi.json").ParseFiles(fileDir + "/" + "openapi.json")
	if err != nil {
		return nil, err
	}

	path := fileDir + "/" + "openapi.json"
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	return response.File{Content: b, ContentType: "application/json"}, nil
}

func SwaggerUIHandler(c *Context) (interface{}, error) {
	fileName := c.PathParam("name")
	if fileName == "" {
		// Read the index.html file
		fileName = "index.html"
	}

	data, err := fs.ReadFile("swagger/" + fileName)
	if err != nil {
		c.Errorf("error while reading the index.html file. err : %v", err)
		return nil, err
	}

	split := strings.Split(fileName, ".")
	if len(split) < 2 {
		return response.File{Content: data}, nil
	}

	ct := mime.TypeByExtension("." + split[1])

	// Return the rendered HTML as a string
	return response.File{Content: data, ContentType: ct}, nil
}
