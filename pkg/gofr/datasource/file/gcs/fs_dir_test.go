package gcs

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource/file"
)

type fakeWriteCloser struct {
	*bytes.Buffer
}

func (*fakeWriteCloser) Close() error {
	return nil
}

type errorWriterCloser struct{}

var (
	errWrite       = errors.New("write error")
	errClose       = errors.New("close error")
	errDirNotFound = errors.New("directory not found")
	errorList      = errors.New("list error")
	errorDelete    = errors.New("delete error")
	errorDirList   = errors.New("dirlist error")
)

func (*errorWriterCloser) Write(_ []byte) (int, error) {
	return 0, errWrite
}

func (*errorWriterCloser) Close() error {
	return errClose
}

type result struct {
	Name  string
	Size  int64
	IsDir bool
}

func TestFileSystem_Mkdir_GCS_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	buf := &bytes.Buffer{}
	fakeWriter := &fakeWriteCloser{Buffer: buf}
	mockGCS.EXPECT().NewWriter(gomock.Any(), "testDir/").Return(fakeWriter)

	config := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: "fake-creds",
		ProjectID:       "test-project",
	}

	fs := &FileSystem{
		conn:    mockGCS,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	err := fs.Mkdir("testDir", 0777)

	require.NoError(t, err)
}

func TestFileSystem_Mkdir_GCS_Error_EmptyName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	config := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: "fake-creds",
		ProjectID:       "test-project",
	}

	fs := &FileSystem{
		conn:    nil,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	err := fs.Mkdir("", 0777)

	require.Error(t, err)
	require.Contains(t, err.Error(), "directory name cannot be empty")
}

func TestFileSystem_Mkdir_GCS_Error_WriteFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	errorWriter := &errorWriterCloser{}
	mockGCS.EXPECT().NewWriter(gomock.Any(), "brokenDir/").Return(errorWriter)

	config := &Config{
		BucketName:      "test-bucket",
		CredentialsJSON: "fake-creds",
		ProjectID:       "test-project",
	}

	fs := &FileSystem{
		conn:    mockGCS,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	err := fs.Mkdir("brokenDir", 0777)

	require.Error(t, err)
	require.Contains(t, err.Error(), "write error")
}
func TestFileSystem_MkdirAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}
	fs := &FileSystem{
		conn:    mockGCS,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), "type",
		gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	mockGCS.EXPECT().NewWriter(gomock.Any(), "foo/").Return(&fakeWriteCloser{Buffer: &bytes.Buffer{}}).AnyTimes()
	mockGCS.EXPECT().NewWriter(gomock.Any(), "foo/bar/").Return(&fakeWriteCloser{Buffer: &bytes.Buffer{}}).AnyTimes()

	err := fs.MkdirAll("foo/bar", 0777)
	require.NoError(t, err, "expected no error during MkdirAll")

	err = fs.MkdirAll("", 0777)
	require.NoError(t, err, "expected no error for empty path")

	mockGCS.EXPECT().NewWriter(gomock.Any(), "errDir/").Return(&errorWriterCloser{}).AnyTimes()

	err = fs.MkdirAll("errDir", 0777)
	require.Error(t, err, "expected error from failed mkdir")
}

func TestFileSystem_RemoveAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn:    mockGCS,
		config:  &Config{BucketName: "test-bucket"},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	mockGCS.EXPECT().ListObjects(gomock.Any(), "fail-list").Return(nil, errorList)

	err := fs.RemoveAll("fail-list")
	require.Error(t, err, "expected error from ListObjects")

	mockGCS.EXPECT().ListObjects(gomock.Any(), "del-error").Return([]string{"del-error/file1"}, nil)
	mockGCS.EXPECT().DeleteObject(gomock.Any(), "del-error/file1").Return(errorDelete)

	err = fs.RemoveAll("del-error")
	require.Error(t, err, "expected error from DeleteObject")

	mockGCS.EXPECT().ListObjects(gomock.Any(), "ok-dir").Return([]string{"ok-dir/file1", "ok-dir/file2"}, nil)
	mockGCS.EXPECT().DeleteObject(gomock.Any(), "ok-dir/file1").Return(nil)
	mockGCS.EXPECT().DeleteObject(gomock.Any(), "ok-dir/file2").Return(nil)

	err = fs.RemoveAll("ok-dir")
	require.NoError(t, err, "expected successful RemoveAll")
}

func TestFileSystem_ChDir_Getwd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		logger:  mockLogger,
		config:  &Config{BucketName: "mybucket"},
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	err := fs.ChDir("any")
	require.Error(t, err)
	require.Equal(t, errCHNDIRNotSupported, err)

	cwd, err := fs.Getwd()

	require.NoError(t, err)
	require.Equal(t, getLocation("mybucket"), cwd)
}

func TestFileSystem_Stat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	fs := &FileSystem{
		conn:    mockGCS,
		logger:  mockLogger,
		config:  &Config{BucketName: "test-bucket"},
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(), "type", gomock.Any(),
		"status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	// File exists
	attrs := &storage.ObjectAttrs{Size: 42, ContentType: "application/pdf", Updated: time.Now()}
	mockGCS.EXPECT().StatObject(gomock.Any(), "exists.txt").Return(attrs, nil)

	info, err := fs.Stat("exists.txt")
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, int64(42), info.Size())

	// File does not exist, directory exists
	mockGCS.EXPECT().StatObject(gomock.Any(), "mydir").Return(nil, storage.ErrObjectNotExist)

	updated := time.Now()
	mockGCS.EXPECT().ListDir(gomock.Any(), "mydir/").Return(
		[]*storage.ObjectAttrs{{Name: "mydir/file", Updated: updated}}, nil, nil)

	info, err = fs.Stat("mydir")
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.IsDir())

	// Directory not found error
	mockGCS.EXPECT().StatObject(gomock.Any(), "otherdir").Return(nil, storage.ErrObjectNotExist)
	mockGCS.EXPECT().ListDir(gomock.Any(), "otherdir/").Return(nil, nil, errorDirList)

	info, err = fs.Stat("otherdir")
	require.Error(t, err)
	require.Nil(t, info)
}

func Test_ReadDir_GCS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	fs := &FileSystem{
		conn:    mockGCS,
		config:  &Config{BucketName: "test-bucket"},
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	mockLogger.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), file.AppFileStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(), "provider", gomock.Any()).AnyTimes()

	for _, tt := range getReadDirTestCases(mockGCS) {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			entries, err := fs.ReadDir(tt.dirPath)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, entries, len(tt.expectedResults))

			for i, entry := range entries {
				require.Equal(t, tt.expectedResults[i].Name, entry.Name())
				require.Equal(t, tt.expectedResults[i].IsDir, entry.IsDir())
				require.Equal(t, tt.expectedResults[i].Size, entry.Size())
			}
		})
	}
}

type readDirTestCase struct {
	name            string
	dirPath         string
	expectedResults []result
	setupMock       func()
	expectError     bool
}

func getReadDirTestCases(mockGCS *MockgcsClient) []readDirTestCase {
	return []readDirTestCase{
		{
			name:    "Valid directory path with files and subdirectory",
			dirPath: "abc/efg",
			expectedResults: []result{
				{"hij", 0, true},
				{"file.txt", 1, false},
			},
			setupMock: func() {
				mockGCS.EXPECT().ListDir(gomock.Any(), "abc/efg").Return(
					[]*storage.ObjectAttrs{{Name: "abc/efg/file.txt", Size: 1}},
					[]string{"abc/efg/hij/"},
					nil,
				)
			},
		},
		{
			name:    "Valid directory path with only subdirectory",
			dirPath: "abc",
			expectedResults: []result{
				{"efg", 0, true},
			},
			setupMock: func() {
				mockGCS.EXPECT().ListDir(gomock.Any(), "abc").Return(
					[]*storage.ObjectAttrs{},
					[]string{"abc/efg/"},
					nil,
				)
			},
		},
		{
			name:            "Directory not found",
			dirPath:         "does-not-exist",
			expectedResults: nil,
			setupMock: func() {
				mockGCS.EXPECT().ListDir(gomock.Any(), "does-not-exist").Return(nil, nil, errDirNotFound)
			},
			expectError: true,
		},
		{
			name:            "Empty directory",
			dirPath:         "empty",
			expectedResults: []result{},
			setupMock: func() {
				mockGCS.EXPECT().ListDir(gomock.Any(), "empty").Return([]*storage.ObjectAttrs{}, nil, nil)
			},
		},
		{
			name:    "Directory with multiple files",
			dirPath: "many/files",
			expectedResults: []result{
				{"file1.txt", 1, false},
				{"file2.txt", 2, false},
			},
			setupMock: func() {
				mockGCS.EXPECT().ListDir(gomock.Any(), "many/files").Return([]*storage.ObjectAttrs{
					{Name: "many/files/file1.txt", Size: 1},
					{Name: "many/files/file2.txt", Size: 2},
				}, nil, nil)
			},
		},
	}
}
