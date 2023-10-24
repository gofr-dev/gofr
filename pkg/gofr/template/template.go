// Package template provides the functionality to render html pages based on the html/templates provided by go
package template

import (
	"bytes"
	"html/template"
	"mime"
	"os"
	"path/filepath"

	"gofr.dev/pkg/errors"
)

type fileType int

const (
	// HTML enum denoting html file
	HTML fileType = iota
	// TEXT enum denoting text file
	TEXT
	// CSV enum denoting csv file
	CSV
	// FILE enum denoting other file types
	FILE
)

// File contains the information about a File and provides functionality to render those files
type File struct {
	// Content holds the file data in slice of bytes
	Content []byte
	// ContentType holds the info about the file type
	ContentType string
}

// Template contains the info about the file and implements a renderer to render the file
type Template struct {
	// Directory denotes the path to the file
	Directory string
	// File denotes the file name
	File string
	// Data has the information that template need to render the file
	Data interface{}
	// Type denotes the file type which is an integer
	Type fileType
}

// Render compiles and executes the template, returning the rendered content.
// It automatically determines the template location and content type.
func (t *Template) Render() ([]byte, error) {
	defaultLocation := t.Directory
	// if the temp location is not specified
	// the default location is taken from root of the project
	if defaultLocation == "" {
		rootLocation, _ := os.Getwd()
		defaultLocation = rootLocation + "/static"
	}

	templ, err := template.New(t.File).ParseFiles(defaultLocation + "/" + t.File)
	if err != nil {
		return nil, errors.FileNotFound{Path: t.Directory, FileName: t.File}
	}

	if t.Data != nil {
		var tpl bytes.Buffer

		err = templ.Execute(&tpl, t.Data)
		if err != nil {
			return nil, err
		}

		return tpl.Bytes(), nil
	}

	path := defaultLocation + "/" + t.File

	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	return b, nil
}

// ContentType returns the content type associated with the template's file type.
func (t *Template) ContentType() string {
	switch t.Type {
	case HTML:
		return "text/html"
	case CSV:
		return "text/csv"
	case TEXT:
		return "text/plain"
	case FILE:
		extn := filepath.Ext(t.File)
		if extn == ".json" {
			return "application/json"
		}

		return mime.TypeByExtension(extn)
	default:
		return "text/plain"
	}
}
