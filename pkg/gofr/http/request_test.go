package http

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/file"
)

func TestParam(t *testing.T) {
	req := NewRequest(httptest.NewRequest("GET", "/abc?a=b", http.NoBody))
	if req.Param("a") != "b" {
		t.Error("Can not parse the request params")
	}
}

func TestBind(t *testing.T) {
	r := httptest.NewRequest("POST", "/abc", strings.NewReader(`{"a": "b", "b": 5}`))
	r.Header.Set("content-type", "application/json")
	req := NewRequest(r)

	x := struct {
		A string `json:"a"`
		B int    `json:"b"`
	}{}

	_ = req.Bind(&x)

	if x.A != "b" || x.B != 5 {
		t.Errorf("Bind error. Got: %v", x)
	}
}

func TestBind_NoContentType(t *testing.T) {
	req := NewRequest(httptest.NewRequest("POST", "/abc", strings.NewReader(`{"a": "b", "b": 5}`)))
	x := struct {
		A string `json:"a"`
		B int    `json:"b"`
	}{}

	_ = req.Bind(&x)

	// The data won't bind so zero values are expected
	if x.A != "" || x.B != 0 {
		t.Errorf("Bind error. Got: %v", x)
	}
}

func Test_GetContext(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "test/hello", http.NoBody)
	r := Request{req: req, pathParams: map[string]string{"key": "hello"}}

	assert.Equal(t, context.Background(), r.Context())
	assert.Equal(t, "http://", r.HostName())
	assert.Equal(t, "hello", r.PathParam("key"))
}

func generateMultipartrequestZip(t *testing.T) *http.Request {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	f, err := os.Open("test.zip")
	if err != nil {
		t.Fatalf("Failed to open test.zip: %v", err)
	}
	defer f.Close()

	zipPart, err := writer.CreateFormFile("zip", "test.zip")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(zipPart, f)
	if err != nil {
		t.Fatalf("Failed to write file to form: %v", err)
	}

	// Close the multipart writer
	writer.Close()

	// Create a new HTTP request with the multipart data
	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("content-type", writer.FormDataContentType())

	return req
}

func Test_bindMultipart(t *testing.T) {
	r := NewRequest(generateMultipartrequestZip(t))
	x := struct {
		X *file.Zip `file:"zip"`
	}{}

	_ = r.bindMultipart(&x)

	assert.NotNil(t, x.X)
	assert.Equal(t, 2, len(x.X.Files))
	assert.Equal(t, []byte("Hello! This is file A.\n"), x.X.Files["a.txt"].Bytes())
	assert.Equal(t, []byte("Hello! This is file B.\n\n"), x.X.Files["b.txt"].Bytes())
}

func Test_bindMultipart_Fails(t *testing.T) {
	// Non-pointer bind
	r := NewRequest(generateMultipartrequestZip(t))
	input := struct {
		file *file.Zip
	}{}

	err := r.bindMultipart(input)
	assert.NotNil(t, err)
	assert.Equal(t, errNonPointerBind, err)

	// unexported field cannot be binded
	err = r.bindMultipart(&input)
	assert.NotNil(t, err)
	assert.Equal(t, errNoFileFound, err)
}
