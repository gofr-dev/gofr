package s3

import (
	"go.uber.org/mock/gomock"
	file_interface "gofr.dev/pkg/gofr/datasource/file"
	"testing"
)

func TestConnect(t *testing.T) {
	cfg := Config{
		"http://localhost:4566",
		"user",
		"gofr-bucket-2",
		"us-east-1",
		"general-purpose",
		"AKIAYHJANQGSVIE2CX7F",
		"ZQaoxNLYiIcdHMwGJJwhPp7ksyyjW27q4eLFTYxZ",
	}
	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	f := fileSystem{
		logger:  mockLogger,
		metrics: mockMetrics,
		config:  &cfg,
	}

	f.Connect()
}

func TestDeleteBucket(t *testing.T) {

}

func Test_CreateTextFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.txt")
		if err != nil {
			t.Error(err)
		}

	})
}

func Test_CreateJSONFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.txt")
		if err != nil {
			t.Error(err)
		}
	})
}

func runS3Test(t *testing.T, testFunc func(fs file_interface.FileSystemProvider)) {
	t.Helper()

	cfg := Config{
		"http://localhost:4566",
		"user",
		"gofr-bucket-2",
		"us-east-1",
		"general-purpose",
		"AKIAYHJANQGSVIE2CX7F",
		"ZQaoxNLYiIcdHMwGJJwhPp7ksyyjW27q4eLFTYxZ",
	}

	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	s3Client := New(&cfg)

	s3Client.UseLogger(mockLogger)
	s3Client.UseMetrics(mockMetrics)

	s3Client.Connect()

	// Run the test function with the initialized file system
	testFunc(s3Client)
}
