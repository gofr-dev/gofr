package file

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"net/url"
	"strconv"
	"testing"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
)

func TestAzureFileOpen(t *testing.T) {
	testcases := []struct {
		azureAcName string
		azureAccKey string
		fileName    string
		blockSize   string
		parallelism string
		mode        Mode
		expErr      error
	}{
		{"alice", "some-random-key", "test.txt", "", "", READ, base64.CorruptInputError(4)},
		{"^@", "c29tZS1yYW5kb20tdGV4dA==", "test.txt", "", "", READWRITE, &url.Error{
			Op:  "parse",
			URL: "https://^@.blob.core.windows.net/container",
			Err: errors.New("net/url: invalid userinfo"),
		}},
		{"bob", "c29tZS1yYW5kb20tdGV4dA==", "test.txt", "abc", "", READWRITE, &strconv.NumError{
			Func: "Atoi",
			Num:  "abc",
			Err:  errors.New("invalid syntax"),
		}},
		{"bob", "c29tZS1yYW5kb20tdGV4dA==", "test.txt", "", "def", READWRITE, &strconv.NumError{
			Func: "Atoi",
			Num:  "def",
			Err:  errors.New("invalid syntax"),
		}},
		{"bob", "c29tZS1yYW5kb20tdGV4dA==", "test.txt", "4194304", "16", READWRITE, nil},
		{"bob", "c29tZS1yYW5kb20tdGV4dA==", "test.txt", "", "", READWRITE, nil},
	}

	for _, v := range testcases {
		c := &config.MockConfig{Data: map[string]string{
			"FILE_STORE":                "AZURE",
			"AZURE_STORAGE_ACCOUNT":     v.azureAcName,
			"AZURE_STORAGE_ACCESS_KEY":  v.azureAccKey,
			"AZURE_STORAGE_CONTAINER":   "container",
			"AZURE_STORAGE_BLOCK_SIZE":  v.blockSize,
			"AZURE_STORAGE_PARALLELISM": v.parallelism,
		}}

		_, err := NewWithConfig(c, "test.txt", READ)
		assert.Equal(t, v.expErr, err)
	}
}

func Test_list_azure(t *testing.T) {
	s := &azure{fileName: "aws.txt", fileMode: READWRITE}
	expErr := ErrListingNotSupported
	_, err := s.list("test")
	assert.Equalf(t, expErr, err, "Test case failed.\nExpected: %v, got: %v", expErr, err)
}

func Test_azure_move(t *testing.T) {
	s := &azure{fileName: "aws.txt", fileMode: READWRITE}
	err := s.move("", "")

	assert.Nil(t, err, "Tests failed")
}

func Test_azure_push(t *testing.T) {
	testPipelineMock := testPipeline{}

	mockBlobURL := azblob.NewBlockBlobURL(url.URL{
		Scheme: "https",
		Host:   "mockstorage.blob.core.windows.net",
		Path:   "/container/mockblob.txt",
	}, testPipelineMock)

	azureFile := &azure{
		fileName:     "azure.txt",
		fileMode:     READWRITE,
		blockBlobURL: mockBlobURL,
		blockSize:    10,
		parallelism:  parallelism,
	}

	expErr := errors.New("test factory invoked")
	localFile := newLocalFile(azureFile.fileName, azureFile.fileMode)

	_ = localFile.Open()
	actualErr := azureFile.push(localFile.FD)

	assert.Equal(t, expErr, actualErr, "Tests failed")

	_ = localFile.Close()
}

func TestValidateAndConvertToUint16(t *testing.T) {
	tests := []struct {
		desc   string
		input  int
		output uint16
	}{
		{"less than 0", -1, uint16(0)},
		{"is 0", 0, uint16(0)},
		{"is max uint16", math.MaxUint16, uint16(math.MaxUint16)},
		{"in uint16 range", 10, uint16(10)},
		{"greater than uint16 range", math.MaxUint16 + 1, uint16(0)},
	}

	for _, tc := range tests {
		result := validateAndConvertToUint16(tc.input)

		assert.Equal(t, tc.output, result)
	}
}

func Test_azure_fetch(t *testing.T) {
	testPipelineMock := testPipeline{}

	mockBlobURL := azblob.NewBlockBlobURL(url.URL{
		Scheme: "https",
		Host:   "mockstorage.blob.core.windows.net",
		Path:   "/container/mockblob.txt",
	}, testPipelineMock)

	azureFile := &azure{
		fileName:     "azure.txt",
		fileMode:     READWRITE,
		blockBlobURL: mockBlobURL,
		blockSize:    10,
		parallelism:  parallelism,
	}
	localFile := newLocalFile(azureFile.fileName, azureFile.fileMode)
	expErr := errors.New("test factory invoked")

	_ = localFile.Open()
	actualErr := azureFile.fetch(localFile.FD)

	assert.Equal(t, expErr, actualErr, "Tests failed.")

	_ = localFile.Close()
}

type testPipeline struct{}

func (tm testPipeline) Do(context.Context, pipeline.Factory, pipeline.Request) (pipeline.Response, error) {
	return nil, errors.New("test factory invoked")
}
