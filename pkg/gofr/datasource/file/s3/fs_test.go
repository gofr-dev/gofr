package s3

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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

// Test_applyClientOptions asserts the flavor→SDK wiring that Connect relies on:
// each resolved profile must set path-style addressing, the base endpoint, and the
// upload-checksum behavior on the concrete *s3.Options. This is what an
// end-to-end connect would otherwise silently depend on.
func Test_applyClientOptions(t *testing.T) {
	const endpoint = "https://example.r2.cloudflarestorage.com"

	tests := []struct {
		name                string
		config              *Config
		wantPathStyle       bool
		wantChecksumWhenReq bool
	}{
		{
			name:                "AWS default keeps path-style and default checksum",
			config:              &Config{Region: "us-east-1"},
			wantPathStyle:       true,
			wantChecksumWhenReq: false,
		},
		{
			name:                "R2 disables upload checksum",
			config:              &Config{Flavor: FlavorR2, Region: "us-east-1"},
			wantPathStyle:       true,
			wantChecksumWhenReq: true,
		},
		{
			name:                "Spaces uses virtual-hosted addressing",
			config:              &Config{Flavor: FlavorSpaces, Region: "nyc3"},
			wantPathStyle:       false,
			wantChecksumWhenReq: false,
		},
		{
			name:                "explicit UsePathStyle override wins",
			config:              &Config{Flavor: FlavorSpaces, Region: "nyc3", UsePathStyle: boolPtr(true)},
			wantPathStyle:       true,
			wantChecksumWhenReq: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &s3.Options{}
			applyClientOptions(resolveProfile(tt.config), endpoint)(o)

			assert.Equal(t, tt.wantPathStyle, o.UsePathStyle, "UsePathStyle")
			require.NotNil(t, o.BaseEndpoint)
			assert.Equal(t, endpoint, *o.BaseEndpoint, "BaseEndpoint")

			if tt.wantChecksumWhenReq {
				assert.Equal(t, aws.RequestChecksumCalculationWhenRequired, o.RequestChecksumCalculation,
					"checksum should be calculated only when required")
			} else {
				assert.NotEqual(t, aws.RequestChecksumCalculationWhenRequired, o.RequestChecksumCalculation,
					"checksum behavior should stay at the SDK default")
			}
		})
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

// Test_CreateFile_FreshPrefix_Success guards the defect-2 fix: creating the
// first object under a brand-new prefix must succeed without any parent
// "directory" existing. S3 prefixes need not pre-exist, so Create must not
// probe with ListObjectsV2 — it writes the object directly.
func Test_CreateFile_FreshPrefix_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	// No ListObjectsV2 expectation: Create must not check for a parent prefix.
	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("test file content")),
		ContentLength: aws.Int64(int64(len("test file content"))),
		ContentType:   aws.String("text/plain"),
		LastModified:  aws.Time(time.Now()),
	}, nil)

	_, err := fs.Create("uploads/3f2a-uuid/report.pdf")
	require.NoError(t, err, "Create under a fresh prefix should succeed")
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
		{
			name:    "Returns only one-level entries using common prefixes",
			dirPath: "abc",
			expectedResults: []result{
				{"root.txt", 10, false},
				{"efg", 0, true},
			},
			setupMock: func() {
				mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, in *s3.ListObjectsV2Input,
						_ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
						// The fix must request a one-level listing: a delimiter is set so nested
						// keys are collapsed into CommonPrefixes instead of streaming every object.
						require.NotNil(t, in.Delimiter, "ReadDir must set a Delimiter for one-level listing")
						require.Equal(t, string(filepath.Separator), *in.Delimiter)
						require.Equal(t, "abc/", *in.Prefix)

						return &s3.ListObjectsV2Output{
							Contents: []types.Object{
								{Key: aws.String("abc/"), Size: aws.Int64(0), LastModified: aws.Time(time.Now())},
								{Key: aws.String("abc/root.txt"), Size: aws.Int64(10), LastModified: aws.Time(time.Now())},
							},
							CommonPrefixes: []types.CommonPrefix{
								{Prefix: aws.String("abc/efg/")},
							},
						}, nil
					})
			},
		},
		{
			name:    "Root directory lists from bucket root and skips nil prefixes",
			dirPath: ".",
			expectedResults: []result{
				{"root.txt", 5, false},
				{"efg", 0, true},
			},
			setupMock: func() {
				mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, in *s3.ListObjectsV2Input,
						_ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
						require.Empty(t, *in.Prefix, "ReadDir(\".\") must list from the bucket root")

						return &s3.ListObjectsV2Output{
							Contents: []types.Object{
								{Key: aws.String("root.txt"), Size: aws.Int64(5), LastModified: aws.Time(time.Now())},
							},
							CommonPrefixes: []types.CommonPrefix{
								{Prefix: nil},
								{Prefix: aws.String("efg/")},
							},
						}, nil
					})
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

// Test_Open_NilOptionalMetadata_NoPanic guards the defect-3 fix. ContentType,
// LastModified and ContentLength are optional response headers; a backend such
// as Cloudflare R2 may omit them, leaving the SDK's pointers nil. Open must not
// dereference them unconditionally.
func Test_Open_NilOptionalMetadata_NoPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	// A nil Content-Length is surfaced as a warning so the degraded handle is diagnosable.
	mocks.mockLogger.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader("content")),
		// ContentType, LastModified and ContentLength deliberately left nil.
	}, nil)

	f, err := fs.Open("no-metadata.bin")
	require.NoError(t, err, "Open must not panic when optional metadata is absent")
	assert.Empty(t, f.(*S3File).contentType, "missing content type defaults to empty string")
	assert.Zero(t, f.(*S3File).size, "missing content length defaults to zero")
}

// Test_CreateFile_NilOptionalMetadata_NoPanic is the Create-side counterpart to
// Test_Open_NilOptionalMetadata_NoPanic.
func Test_CreateFile_NilOptionalMetadata_NoPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	// A nil Content-Length is surfaced as a warning so the degraded handle is diagnosable.
	mocks.mockLogger.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
	mocks.mockS3.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader("")),
		// ContentType, LastModified and ContentLength deliberately left nil.
	}, nil)

	_, err := fs.Create("no-ext-key")
	require.NoError(t, err, "Create must not panic when optional metadata is absent")
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

// Test_RenameFile_ToSameName_LogsSuccess guards against the same-name no-op
// being reported as an ERROR by observability while returning nil to the caller.
func Test_RenameFile_ToSameName_LogsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	logs := make([]FileLog, 0)

	mocks.mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes().Do(func(args ...any) {
		if fl, ok := args[0].(*FileLog); ok {
			logs = append(logs, *fl)
		}
	})

	require.NoError(t, fs.Rename("abcd.json", "abcd.json"))

	require.Len(t, logs, 1)
	require.Equal(t, "RENAME", logs[0].Operation)
	require.Equal(t, statusSuccess, *logs[0].Status)
}

func TestFileSystem_UseLogger(t *testing.T) {
	logger := NewMockLogger(gomock.NewController(t))

	tests := []struct {
		name      string
		logger    any
		expLogger Logger
	}{
		{name: "valid logger is set", logger: logger, expLogger: logger},
		{name: "invalid logger is ignored", logger: "not a logger", expLogger: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &FileSystem{config: defaultTestConfig()}

			fs.UseLogger(tt.logger)

			assert.Equal(t, tt.expLogger, fs.logger)
		})
	}
}

func TestFileSystem_UseMetrics(t *testing.T) {
	metrics := NewMockMetrics(gomock.NewController(t))

	tests := []struct {
		name       string
		metrics    any
		expMetrics Metrics
	}{
		{name: "valid metrics is set", metrics: metrics, expMetrics: metrics},
		{name: "invalid metrics is ignored", metrics: "not metrics", expMetrics: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &FileSystem{config: defaultTestConfig()}

			fs.UseMetrics(tt.metrics)

			assert.Equal(t, tt.expMetrics, fs.metrics)
		})
	}
}

func TestFileSystem_Connect_LoadConfigFails(t *testing.T) {
	dir := t.TempDir()

	// Point the SDK at empty shared config files and select a profile that does not exist,
	// which makes LoadDefaultConfig fail without any network access.
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_PROFILE", "gofr-profile-that-does-not-exist")

	mocks := setupTestMocks(gomock.NewController(t))
	fs := &FileSystem{config: defaultTestConfig(), logger: mocks.mockLogger}

	mocks.mockLogger.EXPECT().Debugf("connecting to S3 bucket: %s", "test-bucket")
	mocks.mockLogger.EXPECT().Errorf("failed to load configuration: %v", gomock.Any())
	mocks.mockLogger.EXPECT().Debug(gomock.Any())

	fs.Connect()

	assert.Nil(t, fs.conn)
}

func TestRemove_DeleteObjectFails_Error(t *testing.T) {
	mocks := setupTestMocks(gomock.NewController(t))
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any())
	mocks.mockLogger.EXPECT().Errorf("Error while deleting file: %v", errMock)
	mocks.mockS3.EXPECT().DeleteObject(gomock.Any(), gomock.Any()).Return(nil, errMock)

	require.ErrorIs(t, fs.Remove("abc.json"), errMock)
}

func Test_RenameFile_Errors(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m *testMocks)
		expErr    error
	}{
		{
			name: "copy object fails",
			setupMock: func(m *testMocks) {
				m.mockS3.EXPECT().CopyObject(gomock.Any(), gomock.Any()).Return(nil, errMock)
			},
			expErr: errMock,
		},
		{
			name: "removing old file fails",
			setupMock: func(m *testMocks) {
				m.mockS3.EXPECT().CopyObject(gomock.Any(), gomock.Any()).Return(&s3.CopyObjectOutput{}, nil)
				m.mockS3.EXPECT().DeleteObject(gomock.Any(), gomock.Any()).Return(nil, errMock)
				m.mockLogger.EXPECT().Errorf("Error while deleting file: %v", errMock)
				m.mockLogger.EXPECT().Debug(gomock.Any())
			},
			expErr: errMock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := setupTestMocks(gomock.NewController(t))
			fs := setupTestFileSystem(mocks, nil)

			mocks.mockLogger.EXPECT().Debug(gomock.Any())
			tt.setupMock(mocks)

			err := fs.Rename("abcd.json", "abc.json")

			require.ErrorIs(t, err, tt.expErr)
		})
	}
}
