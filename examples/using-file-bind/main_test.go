package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain_BindError(t *testing.T) {
	const host = "http://localhost:8300"
	go main()
	time.Sleep(time.Second * 1)

	c := http.Client{}

	req, _ := http.NewRequest(http.MethodPost, host+"/upload", http.NoBody)
	req.Header.Set("content-type", "multipart/form-data")
	resp, err := c.Do(req)

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.NoError(t, err)

	buf, contentType := generateMultiPartBody(t)
	req, _ = http.NewRequest(http.MethodPost, host+"/upload", buf)
	req.Header.Set("content-type", contentType)
	req.ContentLength = int64(buf.Len())

	resp, err = c.Do(req)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func generateMultiPartBody(t *testing.T) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	f, err := os.Open("../../pkg/gofr/testutil/test.zip")
	if err != nil {
		t.Fatalf("Failed to open test.zip: %v", err)
	}
	defer f.Close()

	zipPart, err := writer.CreateFormFile("upload", "test.zip")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(zipPart, f)
	if err != nil {
		t.Fatalf("Failed to write file to form: %v", err)
	}

	fileHeader, err := writer.CreateFormFile("a", "hello.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(fileHeader, bytes.NewReader([]byte(`Test hello!`)))
	if err != nil {
		t.Fatalf("Failed to write file to form: %v", err)
	}

	err = writer.WriteField("name", "test-name")
	if err != nil {
		t.Fatalf("Failed to write name to form: %v", err)
	}

	// Close the multipart writer
	writer.Close()

	return &buf, writer.FormDataContentType()
}
