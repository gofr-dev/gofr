package handler

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/request"
)

func initialiseTest(c config.MockConfig, fileName string) (*gofr.Context, fileHandler) {
	app := gofr.NewWithConfig(&c)
	ctx := gofr.NewContext(nil, nil, app)

	f := New(mockFile{fileName: fileName})

	return ctx, f
}

// Test_Read to test the behavior of Read handler
func Test_Read(t *testing.T) {
	tests := []struct {
		desc     string
		config   config.MockConfig
		fileName string
		response interface{}
		err      error
	}{
		{"successful read", getMockConfigs()["LOCAL"], "success.txt", "Welcome to gofr.dev!", nil},
		{"read error", getMockConfigs()["AZURE"], "err.txt", nil, &fs.PathError{}},
		{"read open error", getMockConfigs()["GCP"], "openErr.txt", nil, &fs.PathError{}},
	}

	for i, tc := range tests {
		ctx, f := initialiseTest(tc.config, tc.fileName)

		resp, err := f.Read(ctx)

		assert.Equal(t, tc.response, resp, "Test case [%d] failed.\n%s", i, tc.desc)

		assert.IsType(t, tc.err, err, "Test case [%d] failed\n%s", i, tc.desc)
	}
}

// Test_Write to test the behavior of Write handler
func Test_Write(t *testing.T) {
	tests := []struct {
		desc     string
		config   config.MockConfig
		fileName string
		response interface{}
		err      error
	}{
		{"write error", getMockConfigs()["LOCAL"], "writeErr.txt", nil, &fs.PathError{}},
		{"successful write", getMockConfigs()["LOCAL"], "success.txt", "File written successfully!", nil},
		{"write open error", getMockConfigs()["GCP"], "openErr.txt", nil, &fs.PathError{}},
	}

	for i, tc := range tests {
		ctx, f := initialiseTest(tc.config, tc.fileName)

		resp, err := f.Write(ctx)

		assert.Equal(t, tc.response, resp, "Test case [%d] failed.\n%s", i, tc.desc)

		assert.IsType(t, tc.err, err, "Test case [%d] failed.\n%s", i, tc.desc)
	}
}

func Test_List(t *testing.T) {
	ctx, f := initialiseTest(getMockConfigs()["LOCAL"], "handler_test.go")

	expRes := []string{"handler.go", "handler_test.go"}
	res, err := f.List(ctx)

	assert.Equal(t, expRes, res, "Test case failed.")

	assert.Nil(t, err, "Test case failed.")
}

// Test_Move to test the behavior of Move handler
func Test_Move(t *testing.T) {
	c := getMockConfigs()["LOCAL"]

	f := fileHandler{mockFile{}}

	testCases := []struct {
		desc   string
		src    string
		dest   string
		expRes interface{}
		expErr error
	}{
		{"Successful file move", "testFileSuccess.txt", "newDir/testFileSuccess.txt",
			"File moved successfully from source:testFileSuccess.txt to destination:newDir/testFileSuccess.txt", nil},
		{"Failure: Empty src flag", "", "newDir/testFileFailureMissing.txt", nil, errors.MissingParam{Param: []string{"src"}}},
		{"Failure: Empty dest flag", "testFileFailureMissing.txt", "", nil, errors.MissingParam{Param: []string{"dest"}}},
		{"Failure: Inaccessible source file", "testFileFailInaccessible.txt", "newDir/testFileFailInaccessible.txt", nil,
			errors.Error("test error")},
		{"Failure: Inaccessible destination path", "testFileFailDestInaccessible.txt", "newDir/testFileFailDestInaccessible.txt", nil,
			errors.Error("test error")},
		{"Failure: Missing source file", "testFileFailureMissing.txt", "newDir/testFileFailureMissing.txt", nil,
			errors.Error("test error")},
	}

	for i, tc := range testCases {
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/dummy?src=%v&dest=%v", tc.src, tc.dest), http.NoBody)
		req := request.NewHTTPRequest(r)
		ctx := gofr.NewContext(nil, req, gofr.NewWithConfig(&c))

		res, err := f.Move(ctx)

		assert.Equalf(t, tc.expRes, res, "Test[%d] Failed: %v", i+1, tc.desc)
		assert.Equalf(t, tc.expErr, err, "Test[%d] Failed: %v", i+1, tc.desc)
	}
}

func getMockConfigs() map[string]config.MockConfig {
	return map[string]config.MockConfig{
		"LOCAL": {Data: map[string]string{"FILE_STORE": "LOCAL"}},
		"AZURE": {Data: map[string]string{"FILE_STORE": "AZURE"}},
		"GCP":   {Data: map[string]string{"FILE_STORE": "GCP"}},
	}
}

// mockFile is the mock implementation of File
type mockFile struct {
	fileName string
}

// Open is the mock implementation of File.Open
func (m mockFile) Open() error {
	if m.fileName == "openErr.txt" {
		return &fs.PathError{}
	}

	return nil
}

// Read is the mock implementation of File.Read
func (m mockFile) Read(b []byte) (int, error) {
	switch m.fileName {
	case "success.txt":
		data := []byte("Welcome to gofr.dev!")
		copy(b, data)

		_, err := m.Write(data)
		if err != nil {
			return 0, err
		}

		return len(data), nil

	case "readErr.txt", "err.txt":
		return 0, &fs.PathError{}
	}

	return 0, nil
}

// Write is the mock implementation of File.Write
func (m mockFile) Write(b []byte) (int, error) {
	switch m.fileName {
	case "success.txt":
		return len(b), nil
	case "writeErr.txt":
		return 0, &fs.PathError{}
	}

	return 0, nil
}

// Close is the mock implementation of File.Close
func (m mockFile) Close() error {
	return nil
}

// List is the mock implementation of File.List
func (m mockFile) List(string) ([]string, error) {
	return []string{"handler.go", "handler_test.go"}, nil
}

// Move is the mock implementation of File.Move
func (m mockFile) Move(src, dest string) error {
	if strings.Contains(src, "Fail") || strings.Contains(dest, "Fail") {
		return errors.Error("test error")
	}

	return nil
}
