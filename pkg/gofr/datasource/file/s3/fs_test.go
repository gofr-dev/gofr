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

// testMocks contains all the mock objects needed for tests.
type testMocks struct {
	mockS3      *Mocks3Client
	mockLogger  *MockLogger
	mockMetrics *MockMetrics
}

// setupTestMocks creates and returns all mock objects needed for testing.
func setupTestMocks(ctrl *gomock.Controller) *testMocks {
	return &testMocks{
		mockS3:      NewMocks3Client(ctrl),
		mockLogger:  NewMockLogger(ctrl),
		mockMetrics: NewMockMetrics(ctrl),
	}
}

// defaultTestConfig returns a default Config for testing.
func defaultTestConfig() *Config {
	return &Config{
		EndPoint:        "https://example.com",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "dummy-access-key",
		SecretAccessKey: "dummy-secret-key",
	}
}

// setupTestFileSystem creates and returns a FileSystem with all required dependencies.
func setupTestFileSystem(mocks *testMocks, config *Config) *FileSystem {
	if config == nil {
		config = defaultTestConfig()
	}

	f := S3File{
		logger:  mocks.mockLogger,
		metrics: mocks.mockMetrics,
		conn:    mocks.mockS3,
	}

	return &FileSystem{
		s3File:  f,
		conn:    mocks.mockS3,
		logger:  mocks.mockLogger,
		config:  config,
		metrics: mocks.mockMetrics,
	}
}

func Test_CreateFile_TxtFile_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("test file content")),
		ContentLength: aws.Int64(int64(len("test file content"))),
		ContentType:   aws.String("text/plain"),
		LastModified:  aws.Time(time.Now()),
	}, nil)

	_, err := fs.Create("abc.txt")
	require.NoError(t, err, "Failed to create file")
}

func Test_CreateFile_WithExistingDirectory_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().
		ListObjectsV2(gomock.Any(), gomock.Any()).
		Return(&s3.ListObjectsV2Output{
			Contents: []types.Object{
				{
					Key:  aws.String("abc.txt"),
					Size: aws.Int64(1),
				},
			},
		}, nil)

	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("test file content")),
		ContentLength: aws.Int64(int64(len("test file content"))),
		ContentType:   aws.String("text/plain"),
		LastModified:  aws.Time(time.Now()),
	}, nil)

	_, err := fs.Create("abc/efg.txt")
	require.NoError(t, err, "Failed to create file with existing directory")
}

func Test_CreateFile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().
		ListObjectsV2(gomock.Any(), gomock.Any()).
		Return(nil, errMock)

	_, err := fs.Create("abc/abc.txt")
	require.Error(t, err, "Expected error during file creation with invalid path")
}

func Test_OpenFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("mock file content")),
		ContentType:   aws.String("text/plain"),
		LastModified:  aws.Time(time.Now()),
		ContentLength: aws.Int64(123),
	}, nil).AnyTimes()

	_, err := fs.OpenFile("abc.json", 0, os.ModePerm)
	require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to open file")
}

func Test_MakingDirectories(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().
		PutObject(gomock.Any(), gomock.Any()).
		Return(&s3.PutObjectOutput{}, nil).Times(3)

	err := fs.MkdirAll("abc/bcd/cfg", os.ModePerm)
	require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error creating directory")
}

func Test_RenameDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	config := defaultTestConfig()
	config.BucketName = "mock-bucket"
	fs := setupTestFileSystem(mocks, config)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().
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

	mocks.mockS3.EXPECT().
		CopyObject(gomock.Any(), gomock.Any()).
		Return(&s3.CopyObjectOutput{}, nil).Times(1)

	mocks.mockS3.EXPECT().
		CopyObject(gomock.Any(), gomock.Any()).
		Return(&s3.CopyObjectOutput{}, nil).Times(1)

	mocks.mockS3.EXPECT().
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

	mocks.mockS3.EXPECT().
		DeleteObjects(gomock.Any(), gomock.Any()).
		Return(&s3.DeleteObjectsOutput{}, nil).Times(1)

	err := fs.Rename("old-dir", "new-dir")
	require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to rename directory")
}

type result struct {
	Name  string
	Size  int64
	IsDir bool
}

func Test_ReadDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

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
				mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
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
				mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
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

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any())
	mocks.mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	name := "testfile.txt"

	mocks.mockS3.EXPECT().DeleteObject(gomock.Any(), gomock.Any()).Return(&s3.DeleteObjectOutput{}, nil).Times(1)

	err := fs.Remove(name)
	require.NoError(t, err, "Remove() failed")
}

func Test_RenameFile_ToNewName_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().CopyObject(gomock.Any(), gomock.Any()).Return(&s3.CopyObjectOutput{}, nil).Times(1)
	mocks.mockS3.EXPECT().DeleteObject(gomock.Any(), gomock.Any()).Return(&s3.DeleteObjectOutput{}, nil).Times(1)

	err := fs.Rename("abcd.json", "abc.json")
	require.NoError(t, err, "Unexpected error when renaming file to new name")
}

func Test_RenameFile_ToSameName_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	err := fs.Rename("abcd.json", "abcd.json")
	require.NoError(t, err, "Unexpected error when renaming file to same name")
}

func Test_RenameFile_WithDifferentExtension_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	err := fs.Rename("abcd.json", "abcd.txt")
	require.Error(t, err, "Expected error when renaming file with different extension")
}

func Test_RenameFile_ToDirectoryPath_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	err := fs.Rename("abcd.json", "abc/abcd.json")
	require.Error(t, err, "Expected error when renaming file to directory path")
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

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
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
	mocks := setupTestMocks(ctrl)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	cfg := &Config{
		EndPoint:        "http://localhost:4566",
		BucketName:      "gofr-bucket-2",
		Region:          "us-east-1",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}
	fs := setupTestFileSystem(mocks, cfg)

	mocks.mockS3.EXPECT().ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
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

func Test_CreateFile_PutObjectFails_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:  aws.String("folder/"),
				Size: aws.Int64(0),
			},
		},
	}, nil)

	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(nil, errMock)

	_, err := fs.Create("folder/test.txt")
	require.Error(t, err, "Expected error when PutObject fails")
}

func Test_CreateFile_GetObjectFails_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:  aws.String("folder/"),
				Size: aws.Int64(0),
			},
		},
	}, nil)

	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(nil, errMock)

	_, err := fs.Create("folder/test.txt")
	require.Error(t, err, "Expected error when GetObject fails after PutObject success")
}

func Test_Open_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any()).Times(1)
	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("test content")),
		ContentType:   aws.String("application/json"),
		LastModified:  aws.Time(time.Now()),
		ContentLength: aws.Int64(12),
	}, nil).Times(1)

	_, err := fs.Open("test.json")
	require.NoError(t, err, "Unexpected error when opening file")
}

func Test_Open_GetObjectFails_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Errorf("failed to retrieve %q: %v", "missing.json", errMock).Times(1)
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).Times(1)
	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(nil, errMock).Times(1)

	_, err := fs.Open("missing.json")
	require.Error(t, err, "Expected error when GetObject fails")
	require.Contains(t, err.Error(), "mocked error", "Expected error to contain mocked error")
}

func Test_RenameDirectory_ListObjectsV2Fails_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(nil, errMock)

	err := fs.Rename("old-dir", "new-dir")
	require.Error(t, err, "Expected error when ListObjectsV2 fails in renameDirectory")
	require.Contains(t, err.Error(), "mocked error", "Expected error to contain mocked error")
}

func Test_Stat_ListObjectsV2Fails_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(nil, errMock)

	_, err := fs.Stat("test-file.txt")
	require.Error(t, err, "Expected error when ListObjectsV2 fails")
	require.Contains(t, err.Error(), "mocked error", "Expected error to contain mocked error")
}
