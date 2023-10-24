package template

import (
	"os"
	"testing"

	"gofr.dev/pkg/log"
)

func createDefaultTemplate() {
	rootDir, _ := os.Getwd()
	logger := log.NewLogger()
	f, err := os.Create(rootDir + "/default.html")

	if err != nil {
		logger.Error(err)
	}

	_, err = f.WriteString(`<!DOCTYPE html>
	<html>
	<head>
	<meta charset="UTF-8">
	<title>{{.Title}}</title>
	</head>
	<body>
	{{range .Items}}<div>{{ . }}</div>{{else}}<div><strong>no rows</strong></div>{{end}}
	</body>
	</html>`)

	if err != nil {
		logger.Error(err)
	} else {
		logger.Info("Template created!")
	}

	err = f.Close()

	if err != nil {
		logger.Error(err)
	}
}

func deleteDefaultTemplate() {
	rootDir, _ := os.Getwd()
	logger := log.NewLogger()
	err := os.Remove(rootDir + "/default.html")

	if err != nil {
		logger.Error(err)
	}
}

func TestTemplate_Render(t1 *testing.T) {
	createDefaultTemplate()

	defer deleteDefaultTemplate()

	templateData := struct {
		Title string
		Items []string
	}{
		Title: "Default Gofr Template",
		Items: []string{
			"Welcome to Gofr",
		},
	}

	type fields struct {
		Directory string
		File      string
		Data      interface{}
		Type      fileType
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"file not passed", fields{"", "", "test data", HTML}, true},
		{"data not passed", fields{"", "", nil, HTML}, true},
		{"wrong data format", fields{"./", "default.html", "test data", HTML}, true},
		{"rendering template success", fields{"./", "default.html", templateData, HTML}, false},
		{"file sent as response", fields{"./", "default.html", nil, HTML}, false},
	}

	for _, tt := range tests {
		t := &Template{
			Directory: tt.fields.Directory,
			File:      tt.fields.File,
			Data:      tt.fields.Data,
			Type:      tt.fields.Type,
		}

		_, err := t.Render()
		if (err != nil) != tt.wantErr {
			t1.Errorf("TestCase %v: error = %v, wantErr %v", tt.name, err, tt.wantErr)
			return
		}
	}
}

func TestTemplate_ContentType(t1 *testing.T) {
	tests := []struct {
		Type fileType
		want string
	}{
		{10, "text/plain"},
		{CSV, "text/csv"},
		{HTML, "text/html"},
		{TEXT, "text/plain"},
	}
	for _, tt := range tests {
		t := &Template{
			Type: tt.Type,
		}
		if got := t.ContentType(); got != tt.want {
			t1.Errorf("ContentType() = %v, want %v", got, tt.want)
		}
	}
}

func TestTemplate_fileContentType(t1 *testing.T) {
	testCases := []struct {
		fileName    string
		contentType string
	}{
		{"test.txt", "text/plain; charset=utf-8"},
		{"abc.txt", "text/plain; charset=utf-8"},
		{"default.html", "text/html; charset=utf-8"},
		{"sample.css", "text/css; charset=utf-8"},
		{"sample.svg", "image/svg+xml"},
		{"sample.js", "text/javascript; charset=utf-8"},
		{"image.jpeg", "image/jpeg"},
		{"openapi.json", "application/json"},
	}

	for i, tc := range testCases {
		t := &Template{
			File: tc.fileName,
			Type: FILE,
		}

		_, _ = os.Create(tc.fileName)

		if got := t.ContentType(); got != tc.contentType {
			t1.Errorf("[TESTCASE%d]fileContentType() = %v, want %v", i+1, got, tc.contentType)
		}

		_ = os.Remove(tc.fileName)
	}
}
