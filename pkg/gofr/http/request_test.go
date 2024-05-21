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

func TestBind_FileSuccess(t *testing.T) {
	r := NewRequest(generateMultipartrequestZip(t))
	x := struct {
		// Zip file bind for zip struct
		Zip file.Zip `file:"zip"`

		// Zip file bind for zip pointer
		ZipPtr *file.Zip `file:"zip"`

		// FileHeader multipart.FileHeader bind(value)
		FileHeader multipart.FileHeader `file:"hello"`

		// FileHeaderPtr mulitpart.FileHeader bind for pointer
		FileHeaderPtr *multipart.FileHeader `file:"hello"`

		// Skip bind
		Skip *file.Zip `file:"-"`

		// Incompatible type cannot be bound
		Incompatible string `file:"hello"`

		// File not in multipart form
		FileNotPresent *multipart.FileHeader `file:"text"`
	}{}

	err := r.Bind(&x)
	assert.Nil(t, err)

	// Assert zip file bind
	assert.Equal(t, 2, len(x.Zip.Files))
	assert.Equal(t, "Hello! This is file A.\n", string(x.Zip.Files["a.txt"].Bytes()))
	assert.Equal(t, "Hello! This is file B.\n\n", string(x.Zip.Files["b.txt"].Bytes()))

	// Assert zip file bind for pointer
	assert.NotNil(t, x.ZipPtr)
	assert.Equal(t, 2, len(x.ZipPtr.Files))
	assert.Equal(t, "Hello! This is file A.\n", string(x.ZipPtr.Files["a.txt"].Bytes()))
	assert.Equal(t, "Hello! This is file B.\n\n", string(x.ZipPtr.Files["b.txt"].Bytes()))

	// Assert FileHeader struct type
	assert.Equal(t, "hello.txt", x.FileHeader.Filename)

	f, err := x.FileHeader.Open()
	assert.Nil(t, err)
	assert.NotNil(t, f)

	content, err := io.ReadAll(f)
	assert.Nil(t, err)
	assert.Equal(t, "Test hello!", string(content))

	// Assert FileHeader pointer type
	assert.NotNil(t, x.FileHeader)
	assert.Equal(t, "hello.txt", x.FileHeader.Filename)

	f, err = x.FileHeader.Open()
	assert.Nil(t, err)
	assert.NotNil(t, f)

	content, err = io.ReadAll(f)
	assert.Nil(t, err)
	assert.Equal(t, "Test hello!", string(content))

	// Assert skipped field
	assert.Nil(t, x.Skip)

	// Assert incompatible
	assert.Equal(t, "", x.Incompatible)

	// Assert file not present
	assert.Nil(t, x.FileNotPresent)
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

	f, err := os.Open("../testutil/test.zip")
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

	fileHeader, err := writer.CreateFormFile("hello", "hello.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(fileHeader, bytes.NewReader([]byte(`Test hello!`)))
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

func Test_bindMultipart_Fail_ParseMultiPart(t *testing.T) {
	r := NewRequest(generateMultipartrequestZip(t))
	input2 := struct {
		File *file.Zip `file:"zip"`
	}{}

	// Call the multipart reader to handle form from a multipart reader
	// This is called to invoke error while parsing Multipart form in bind
	_, _ = r.req.MultipartReader()

	err := r.bindMultipart(&input2)
	assert.NotNil(t, err)
	assert.Equal(t, "http: multipart handled by MultipartReader", err.Error())
}
