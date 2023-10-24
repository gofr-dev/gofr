package responder

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/template"
	"gofr.dev/pkg/log"
)

// TestCMD_Respond tests the Respond function
func TestCMD_Respond(t *testing.T) {
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

	logger := log.NewMockLogger(io.Discard)
	tmpFile, _ := os.CreateTemp("", "fake-stdout.*")

	defer os.Remove(tmpFile.Name())

	os.Stdout = tmpFile

	testCases := []struct {
		desc string
		data interface{}
		err  error
		want string
	}{
		{"case when data is template.Template and file data is a proper template",
			template.Template{Directory: "./", File: "default.html",
				Data: templateData, Type: template.HTML}, nil, "<!DOCTYPE html>"},
		{"case when data is template.File", template.File{Content: []byte(`<html></html>`),
			ContentType: "text/html"}, nil, "<html></html>"},
		{"case when data is a string and not template", "test data", nil, "test data"},
		{"case when data is template.Template and file data is a string", template.Template{Directory: "./",
			File: "default.html", Data: "test data", Type: template.HTML},
			nil, "template: default.html:5:10: executing"},
	}
	for i, tc := range testCases {
		h := CMD{logger}
		h.Respond(tc.data, tc.err)

		outputBytes, _ := os.ReadFile(tmpFile.Name())

		output := strings.TrimSpace(string(outputBytes))

		assert.Contains(t, output, tc.want, "Test[%d] Failed: %v", i+1, tc.desc)
	}
}

// TestCMD_Respond_LogError tests the logger that is being added in case of CMD application
func TestCMD_Respond_LogError(t *testing.T) {
	createDefaultTemplate()

	defer deleteDefaultTemplate()

	testcases := []struct {
		desc   string
		data   interface{}
		err    error
		expLog string
	}{
		{"case when data is nil and throws error which is getting logged", nil, errors.Error("test error"), "test error"},
		{"case when data is template.Template and file data is a proper template", template.Template{Directory: "./", File: "default.html",
			Data: "test data", Type: template.HTML}, nil, "\"Name\":\"default.html\""},
	}

	for i, tc := range testcases {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)

		h := CMD{logger}

		h.Respond(tc.data, tc.err)

		t.Logf("log %v", b.String())

		assert.Contains(t, b.String(), tc.expLog, "[TEST %v] Failed\n%v", i, tc.desc)
	}
}
