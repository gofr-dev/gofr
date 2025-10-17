package s3

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errMock = errors.New("mocked error")

func Test_CreateFile(t *testing.T) {
	type testCase struct {
		name        string
		createPath  string
		setupMocks  func()
		expectError bool
		isRoot      bool
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl) // Replace with the actual generated mock for the S3 client.
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	tests := []testCase{
		{name: "Create txt file", createPath: "abc.txt",
			setupMocks: func() {
				mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
				mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
					Body:          io.NopCloser(strings.NewReader("test file content")),
					ContentLength: aws.Int64(int64(len("test file content"))),
					ContentType:   aws.String("text/plain"),
					LastModified:  aws.Time(time.Now()),
				}, nil)
			},
			expectError: false, isRoot: true},
		{name: "Create file with invalid path", createPath: "abc/abc.txt",
			setupMocks: func() {
				mockS3.EXPECT().
					ListObjectsV2(gomock.Any(), gomock.Any()).
					Return(nil, errMock)
			},
			expectError: true, isRoot: false},
		{name: "Create valid file with directory existing", createPath: "abc/efg.txt",
			setupMocks: func() {
				mockS3.EXPECT().
					ListObjectsV2(gomock.Any(), gomock.Any()).
					Return(&s3.ListObjectsV2Output{
						Contents: []types.Object{
							{
								Key:  aws.String("abc.txt"),
								Size: aws.Int64(1),
							},
						},
					}, nil)

				mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
				mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
					Body:          io.NopCloser(strings.NewReader("test file content")),
					ContentLength: aws.Int64(int64(len("test file content"))),
					ContentType:   aws.String("text/plain"),
					LastModified:  aws.Time(time.Now()),
				}, nil)
			},
			expectError: false, isRoot: false},
	}

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	for i, tt := range tests {
		tt.setupMocks()
		_, err := fs.Create(tt.createPath)

		if tt.expectError {
			require.Error(t, err, "TEST[%d] Failed. Desc %v", i, "Expected error during file creation")
			return
		}

		require.NoError(t, err, "TEST[%d] Failed. Desc %v", i, "Failed to create file")
	}
}

func Test_CreateFile_ErrorCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	tests := []struct {
		name        string
		createPath  string
		setupMocks  func()
		expectError bool
	}{
		{
			name:       "PutObject_Fails",
			createPath: "folder/test.txt",
			setupMocks: func() {
				// Mock ListObjectsV2 to succeed (for parent path check)
				mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{
						{
							Key:  aws.String("folder/"),
							Size: aws.Int64(0),
						},
					},
				}, nil)
				// Mock PutObject to fail
				mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(nil, errMock)
			},
			expectError: true,
		},
		{
			name:       "GetObject_Fails_After_PutObject_Success",
			createPath: "folder/test.txt",
			setupMocks: func() {
				// Mock ListObjectsV2 to succeed (for parent path check)
				mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{
						{
							Key:  aws.String("folder/"),
							Size: aws.Int64(0),
						},
					},
				}, nil)
				// Mock PutObject to succeed
				mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
				// Mock GetObject to fail
				mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(nil, errMock)
			},
			expectError: true,
		},
	}

	// Don't mock logger expectations - let the actual logger calls happen
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			_, err := fs.Create(tt.createPath)

			if tt.expectError {
				require.Error(t, err, "Expected error for case: %s", tt.name)
			} else {
				require.NoError(t, err, "Unexpected error for case: %s", tt.name)
			}
		})
	}
}

func Test_OpenFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl) // Replace with the actual generated mock for the S3 client.
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	tests := []struct {
		name          string
		fileName      string
		setupMocks    func()
		expectError   bool
		expectedError string
	}{
		{
			name:     "Success_OpenFile",
			fileName: "abc.json",
			setupMocks: func() {
				mockLogger.EXPECT().Debug(gomock.Any()).Times(1) // For the deferred stat log
				mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
					Body:          io.NopCloser(strings.NewReader("mock file content")), // Mock file content
					ContentType:   aws.String("text/plain"),                             // Mock content type
					LastModified:  aws.Time(time.Now()),                                 // Mock last modified time
					ContentLength: aws.Int64(123),                                       // Mock file size in bytes
				}, nil).Times(1)
			},
			expectError: false,
		},
		{
			name:     "Error_GetObjectFails",
			fileName: "nonexistent.json",
			setupMocks: func() {
				mockLogger.EXPECT().Errorf("failed to retrieve %q: %v", "nonexistent.json", errMock).Times(1)
				mockLogger.EXPECT().Debug(gomock.Any()).Times(1) // For the deferred stat log
				mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(nil, errMock).Times(1)
			},
			expectError:   true,
			expectedError: "mocked error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock expectations for each test
			ctrl.Finish()
			ctrl = gomock.NewController(t)
			mockS3 = NewMocks3Client(ctrl)
			mockLogger = NewMockLogger(ctrl)
			mockMetrics = NewMockMetrics(ctrl)

			// Update the FileSystem with new mocks
			fs.conn = mockS3
			fs.logger = mockLogger
			fs.metrics = mockMetrics

			// Setup mocks for this test case
			tt.setupMocks()

			// Execute the test
			_, err := fs.OpenFile(tt.fileName, 0, os.ModePerm)

			if tt.expectError {
				require.Error(t, err, "Expected error but got none for case: %s", tt.name)

				if tt.expectedError != "" {
					require.Contains(t, err.Error(), tt.expectedError, "Expected error to contain %q, got %q", tt.expectedError, err.Error())
				}
			} else {
				require.NoError(t, err, "Unexpected error for case: %s", tt.name)
			}
		})
	}
}

func Test_Open(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	tests := []struct {
		name          string
		fileName      string
		setupMocks    func()
		expectError   bool
		expectedError string
	}{
		{
			name:     "Success_OpenFile",
			fileName: "test.json",
			setupMocks: func() {
				mockLogger.EXPECT().Debug(gomock.Any()).Times(1) // For the deferred stat log
				mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
					Body:          io.NopCloser(strings.NewReader("test content")),
					ContentType:   aws.String("application/json"),
					LastModified:  aws.Time(time.Now()),
					ContentLength: aws.Int64(12),
				}, nil).Times(1)
			},
			expectError: false,
		},
		{
			name:     "Error_GetObjectFails",
			fileName: "missing.json",
			setupMocks: func() {
				mockLogger.EXPECT().Errorf("failed to retrieve %q: %v", "missing.json", errMock).Times(1)
				mockLogger.EXPECT().Debug(gomock.Any()).Times(1) // For the deferred stat log
				mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(nil, errMock).Times(1)
			},
			expectError:   true,
			expectedError: "mocked error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock expectations for each test
			ctrl.Finish()
			ctrl = gomock.NewController(t)
			mockS3 = NewMocks3Client(ctrl)
			mockLogger = NewMockLogger(ctrl)
			mockMetrics = NewMockMetrics(ctrl)

			// Update the FileSystem with new mocks
			fs.conn = mockS3
			fs.logger = mockLogger
			fs.metrics = mockMetrics

			// Setup mocks for this test case
			tt.setupMocks()

			// Execute the test
			_, err := fs.Open(tt.fileName)

			if tt.expectError {
				require.Error(t, err, "Expected error but got none for case: %s", tt.name)

				if tt.expectedError != "" {
					require.Contains(t, err.Error(), tt.expectedError, "Expected error to contain %q, got %q", tt.expectedError, err.Error())
				}
			} else {
				require.NoError(t, err, "Unexpected error for case: %s", tt.name)
			}
		})
	}
}

func Test_MakingDirectories(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mockS3.EXPECT().
		PutObject(gomock.Any(), gomock.Any()).
		Return(&s3.PutObjectOutput{}, nil).Times(3)

	err := fs.MkdirAll("abc/bcd/cfg", os.ModePerm)
	require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error creating directory")
}

func Test_RenameDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "mock-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mockS3.EXPECT().
		ListObjectsV2(gomock.Any(), gomock.Any()).
		Return(&s3.ListObjectsV2Output{
			Contents: []types.Object{
				{
					Key: aws.String("old-dir/file1.txt"),
				},
				{
					Key: aws.String("old-dir/file2.txt"),
				},
			},
		}, nil).Times(1)

	mockS3.EXPECT().
		CopyObject(gomock.Any(), gomock.Any()).
		Return(&s3.CopyObjectOutput{}, nil).Times(1)

	mockS3.EXPECT().
		CopyObject(gomock.Any(), gomock.Any()).
		Return(&s3.CopyObjectOutput{}, nil).Times(1)

	mockS3.EXPECT().
		ListObjectsV2(gomock.Any(), gomock.Any()).
		Return(&s3.ListObjectsV2Output{
			Contents: []types.Object{
				{
					Key: aws.String("old-dir/file1.txt"),
				},
				{
					Key: aws.String("old-dir/file2.txt"),
				},
			},
		}, nil).Times(1)

	mockS3.EXPECT().
		DeleteObjects(gomock.Any(), gomock.Any()).
		Return(&s3.DeleteObjectsOutput{}, nil).Times(1)

	err := fs.Rename("old-dir", "new-dir")
	require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to rename directory")
}

func Test_renameDirectory_ErrorCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	// Mock ListObjectsV2 to fail
	mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(nil, errMock)

	// Don't mock logger expectations - let the actual logger calls happen
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	// Execute the test by calling Rename with directory paths (no extension)
	err := fs.Rename("old-dir", "new-dir")

	// Verify error is returned
	require.Error(t, err, "Expected error when ListObjectsV2 fails in renameDirectory")
	require.Contains(t, err.Error(), "mocked error", "Expected error to contain the mocked error")
}

type result struct {
	Name  string
	Size  int64
	IsDir bool
}

func Test_ReadDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockS3 := NewMocks3Client(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{logger: mockLogger, metrics: mockMetrics, conn: mockS3}
	fs := &FileSystem{s3File: f, conn: mockS3, logger: mockLogger, config: config, metrics: mockMetrics}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	tests := []struct {
		name            string
		dirPath         string
		expectedResults []result
		setupMock       func()
	}{
		{
			name:    "Valid directory path with files and subdirectory",
			dirPath: "abc/efg",
			expectedResults: []result{
				{"file.txt", 1, false},
				{"hij", 0, true},
			},
			setupMock: func() {
				mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{
						{Key: aws.String("abc/efg/"), Size: aws.Int64(0), LastModified: aws.Time(time.Now())},
						{Key: aws.String("abc/efg/file.txt"), Size: aws.Int64(1), LastModified: aws.Time(time.Now())},
						{Key: aws.String("abc/efg/hij/"), Size: aws.Int64(0), LastModified: aws.Time(time.Now())},
					},
				}, nil)
			},
		},
		{
			name:    "Valid directory path with only subdirectory",
			dirPath: "abc",
			expectedResults: []result{
				{"efg", 0, true},
			},
			setupMock: func() {
				mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{
						{Key: aws.String("abc/"), Size: aws.Int64(0), LastModified: aws.Time(time.Now())},
						{Key: aws.String("abc/efg/"), Size: aws.Int64(0), LastModified: aws.Time(time.Now())},
					},
				}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runReadDirTest(t, fs, tt.dirPath, tt.expectedResults, tt.setupMock)
		})
	}
}

func runReadDirTest(t *testing.T, fs *FileSystem, dirPath string, expectedResults []result, setupMock func()) {
	t.Helper()

	if setupMock != nil {
		setupMock()
	}

	res, err := fs.ReadDir(dirPath)
	require.NoError(t, err, "Error reading directory")

	results := make([]result, 0)

	for _, entry := range res {
		results = append(results, result{entry.Name(), entry.Size(), entry.IsDir()})
	}

	assert.Equal(t, expectedResults, results, "Mismatch in results for path: %v", dirPath)
}

func TestRemove(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl) // Replace with the actual generated mock for the S3 client.
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any())
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	name := "testfile.txt"

	mockS3.EXPECT().DeleteObject(gomock.Any(), gomock.Any()).Return(&s3.DeleteObjectOutput{}, nil).Times(1)

	err := fs.Remove(name)
	if err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
}

func Test_RenameFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl) // Replace with the actual generated mock for the S3 client.
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	tests := []struct {
		name          string
		initialName   string
		newName       string
		expectedError bool
	}{
		{
			name:          "Rename file to new name",
			initialName:   "abcd.json",
			newName:       "abc.json",
			expectedError: false,
		},
		{
			name:          "Rename file with different extension",
			initialName:   "abcd.json",
			newName:       "abcd.txt",
			expectedError: true,
		},
		{
			name:          "Rename file to same name",
			initialName:   "abcd.json",
			newName:       "abcd.json",
			expectedError: false,
		},
		{
			name:          "Rename file to directory path (unsupported)",
			initialName:   "abcd.json",
			newName:       "abc/abcd.json",
			expectedError: true,
		},
	}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{}, nil).AnyTimes()
	mockS3.EXPECT().CopyObject(gomock.Any(), gomock.Any()).Return(&s3.CopyObjectOutput{}, nil).Times(1)
	mockS3.EXPECT().DeleteObject(gomock.Any(), gomock.Any()).Return(&s3.DeleteObjectOutput{}, nil).Times(1)

	// Iterate through each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fs.Rename(tt.initialName, tt.newName)

			if tt.expectedError {
				require.Error(t, err, "Expected error but got none for case: %s", tt.name)
			} else {
				require.NoError(t, err, "Unexpected error for case: %s", tt.name)
			}
		})
	}
}

func Test_StatFile(t *testing.T) {
	tm := time.Now()

	type result struct {
		name  string
		size  int64
		isDir bool
	}

	expectedResponse := result{
		name:  "file.txt",
		size:  1,
		isDir: false,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl) // Replace with the actual generated mock for the S3 client.
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:          aws.String("file.txt"),
				Size:         aws.Int64(1),
				LastModified: aws.Time(tm),
			},
		},
	}, nil).AnyTimes()

	res, err := fs.Stat("dir1/dir2/file.txt")

	response := result{res.Name(), res.Size(), res.IsDir()}

	require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error getting file info")
	assert.Equal(t, expectedResponse, response, "Mismatch in results for path: %v", expectedResponse.name)
}

func Test_StatDirectory(t *testing.T) {
	type result struct {
		name  string
		size  int64
		isDir bool
	}

	expectedResponse := result{
		name:  "dir2",
		size:  1,
		isDir: true,
	}

	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockConn := NewMocks3Client(ctrl)

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	cfg := Config{
		"http://localhost:4566",
		"gofr-bucket-2",
		"us-east-1",
		"test",
		"test",
	}
	f := S3File{
		conn:    mockConn,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	fs := FileSystem{
		s3File:  f,
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockConn,
		config:  &cfg,
	}

	mockConn.EXPECT().ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String("gofr-bucket-2"),
		Prefix: aws.String("dir1/dir2"),
	}).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:          aws.String("dir1/dir2/"),
				Size:         aws.Int64(0),
				LastModified: aws.Time(time.Now()),
			},
			{
				Key:          aws.String("dir1/dir2/file.txt"),
				Size:         aws.Int64(1),
				LastModified: aws.Time(time.Now()),
			},
		},
	}, nil)

	res, err := fs.Stat("dir1/dir2")

	response := result{res.Name(), res.Size(), res.IsDir()}

	require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error getting directory stats")
	assert.Equal(t, expectedResponse, response, "Mismatch in results for path: %v", expectedResponse.name)
}

func Test_Stat_ErrorCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock S3 client
	mockS3 := NewMocks3Client(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Define the configuration for the S3 package
	config := &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}

	f := S3File{
		logger:  mockLogger,
		metrics: mockMetrics,
		conn:    mockS3,
	}

	fs := &FileSystem{
		s3File:  f,
		conn:    mockS3,
		logger:  mockLogger,
		config:  config,
		metrics: mockMetrics,
	}

	// Mock ListObjectsV2 to fail
	mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(nil, errMock)

	// Don't mock logger expectations - let the actual logger calls happen
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	// Execute the test
	_, err := fs.Stat("test-file.txt")

	// Verify error is returned
	require.Error(t, err, "Expected error when ListObjectsV2 fails")
	require.Contains(t, err.Error(), "mocked error", "Expected error to contain the mocked error")
}
