package gcs

import (
	"bytes"
	"errors"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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

func Test_Mkdir_GCS(t *testing.T) {
	type testCase struct {
		name        string
		dirName     string
		setupMocks  func(mockGCS *MockgcsClient)
		expectError bool
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGCS := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

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

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

	tests := []testCase{
		{
			name:    "successfully create directory",
			dirName: "testDir",
			setupMocks: func(m *MockgcsClient) {
				buf := &bytes.Buffer{}
				fakeWriter := &fakeWriteCloser{Buffer: buf}
				m.EXPECT().NewWriter(gomock.Any(), "testDir/").Return(fakeWriter)
			},
			expectError: false,
		},
		{
			name:    "fail when directory name is empty",
			dirName: "",
			setupMocks: func(_ *MockgcsClient) {
				// No mock needed for empty dir
			},
			expectError: true,
		},
		{
			name:    "fail when GCS write fails",
			dirName: "brokenDir",
			setupMocks: func(m *MockgcsClient) {
				errorWriter := &errorWriterCloser{}
				m.EXPECT().NewWriter(gomock.Any(), "brokenDir/").Return(errorWriter)
			},
			expectError: true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks(mockGCS)

			err := fs.Mkdir(tt.dirName, 0777)

			if tt.expectError {
				require.Error(t, err, "Test %d (%s): expected an error", i, tt.name)
			} else {
				require.NoError(t, err, "Test %d (%s): expected no error", i, tt.name)
			}
		})
	}
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

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

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
