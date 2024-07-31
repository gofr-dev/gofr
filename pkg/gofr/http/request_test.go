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
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/file"
)

func TestParam(t *testing.T) {
	req := NewRequest(httptest.NewRequest(http.MethodGet, "/abc?a=b", http.NoBody))
	if req.Param("a") != "b" {
		t.Error("Can not parse the request params")
	}
}

func TestBind(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader(`{"a": "b", "b": 5}`))
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
	r := NewRequest(generateMultipartRequestZip(t))
	x := struct {
		// Zip file bind for zip struct
		Zip file.Zip `file:"zip"`

		// Zip file bind for zip pointer
		ZipPtr *file.Zip `file:"zip"`

		// FileHeader multipart.FileHeader bind(value)
		FileHeader multipart.FileHeader `file:"hello"`

		// FileHeaderPtr multipart.FileHeader bind for pointer
		FileHeaderPtr *multipart.FileHeader `file:"hello"`

		// Skip bind
		Skip *file.Zip `file:"-"`

		// Incompatible type cannot be bound
		Incompatible string `file:"hello"`

		// File not in multipart form
		FileNotPresent *multipart.FileHeader `file:"text"`

		// Additional form fields
		StringField string  `form:"stringField"`
		IntField    int     `form:"intField"`
		FloatField  float64 `form:"floatField"`
		BoolField   bool    `form:"boolField"`
	}{}

	err := r.Bind(&x)
	require.NoError(t, err)

	// Assert zip file bind
	assert.Len(t, x.Zip.Files, 2)
	assert.Equal(t, "Hello! This is file A.\n", string(x.Zip.Files["a.txt"].Bytes()))
	assert.Equal(t, "Hello! This is file B.\n\n", string(x.Zip.Files["b.txt"].Bytes()))

	// Assert zip file bind for pointer
	assert.NotNil(t, x.ZipPtr)
	assert.Len(t, x.ZipPtr.Files, 2)
	assert.Equal(t, "Hello! This is file A.\n", string(x.ZipPtr.Files["a.txt"].Bytes()))
	assert.Equal(t, "Hello! This is file B.\n\n", string(x.ZipPtr.Files["b.txt"].Bytes()))

	// Assert FileHeader struct type
	assert.Equal(t, "hello.txt", x.FileHeader.Filename)

	f, err := x.FileHeader.Open()
	require.NoError(t, err)
	assert.NotNil(t, f)

	content, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "Test hello!", string(content))

	// Assert FileHeader pointer type
	assert.NotNil(t, x.FileHeader)
	assert.Equal(t, "hello.txt", x.FileHeader.Filename)

	f, err = x.FileHeader.Open()
	require.NoError(t, err)
	assert.NotNil(t, f)

	content, err = io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "Test hello!", string(content))

	// Assert skipped field
	assert.Nil(t, x.Skip)

	// Assert incompatible
	assert.Equal(t, "", x.Incompatible)

	// Assert file not present
	assert.Nil(t, x.FileNotPresent)

	// Assert additional form fields
	assert.Equal(t, "testString", x.StringField)
	assert.Equal(t, 123, x.IntField)
	assert.InEpsilon(t, 123.456, x.FloatField, 0.01)

	assert.True(t, x.BoolField)
}

func TestBind_NoContentType(t *testing.T) {
	req := NewRequest(httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader(`{"a": "b", "b": 5}`)))
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

func generateMultipartRequestZip(t *testing.T) *http.Request {
	t.Helper()

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

	// Add non-file fields
	err = writer.WriteField("stringField", "testString")
	require.NoError(t, err)

	err = writer.WriteField("intField", "123")
	require.NoError(t, err)

	err = writer.WriteField("floatField", "123.456")
	require.NoError(t, err)

	err = writer.WriteField("boolField", "true")
	require.NoError(t, err)

	// Close the multipart writer
	writer.Close()

	// Create a new HTTP request with the multipart data
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("content-type", writer.FormDataContentType())

	return req
}

func Test_bindMultipart_Fails(t *testing.T) {
	// Non-pointer bind
	r := NewRequest(generateMultipartRequestZip(t))
	input := struct {
		file *file.Zip
	}{}

	err := r.bindMultipart(input)
	require.Error(t, err)
	assert.Equal(t, errNonPointerBind, err)

	// unexported field cannot be binded
	err = r.bindMultipart(&input)
	require.ErrorIs(t, err, errNoFileFound)
}

func Test_bindMultipart_Fail_ParseMultiPart(t *testing.T) {
	r := NewRequest(generateMultipartRequestZip(t))
	input2 := struct {
		File *file.Zip `file:"zip"`
	}{}

	// Call the multipart reader to handle form from a multipart reader
	// This is called to invoke error while parsing Multipart form in bind
	_, _ = r.req.MultipartReader()

	err := r.bindMultipart(&input2)
	require.ErrorContains(t, err, "http: multipart handled by MultipartReader")
}
