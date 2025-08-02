package gcs

import (
	"errors"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func Test_CreateFile(t *testing.T) {
	type testCase struct {
		name        string
		createPath  string
		setupMocks  func(mockGCS *MockgcsClient)
		expectError bool
		isRoot      bool
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
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	tests := []testCase{
		{
			name:       "create file at root level",
			createPath: "abc.txt",
			setupMocks: func(m *MockgcsClient) {
				m.EXPECT().ListObjects(gomock.Any(), ".").Return([]string{}, nil)
				m.EXPECT().ListObjects(gomock.Any(), "abc.txt").Return([]string{}, nil)
				m.EXPECT().NewWriter(gomock.Any(), "abc.txt").Return(&storage.Writer{})

			},
			expectError: false,
			isRoot:      true,
		},
		{
			name:       "fail when parent directory does not exist",
			createPath: "abc/abc.txt",
			setupMocks: func(m *MockgcsClient) {
				m.EXPECT().ListObjects(gomock.Any(), "abc/").Return(nil, errors.New("errMock"))
			},
			expectError: true,
			isRoot:      false,
		},
		{
			name:       "create file inside existing directory",
			createPath: "abc/efg.txt",
			setupMocks: func(m *MockgcsClient) {
				// parent path "abc/" exists
				m.EXPECT().ListObjects(gomock.Any(), "abc/").Return([]string{"abc/.keep"}, nil)
				// filename does not exist
				m.EXPECT().ListObjects(gomock.Any(), "abc/efg.txt").Return([]string{}, nil)
				m.EXPECT().NewWriter(gomock.Any(), "abc/efg.txt").Return(&storage.Writer{})
			},
			expectError: false,
			isRoot:      false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks(mockGCS)

			file, err := fs.Create(tt.createPath)

			if tt.expectError {
				require.Error(t, err, "Test %d (%s): expected an error", i, tt.name)
				return
			}

			require.NoError(t, err, "Test %d (%s): expected no error", i, tt.name)
			require.NotNil(t, file)
		})
	}
}
func Test_Remove_GCS(t *testing.T) {
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

	// Expectations
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mockGCS.EXPECT().
		DeleteObject(gomock.Any(), "abc/a1.txt").
		Return(nil).
		Times(1)

	err := fs.Remove("abc/a1.txt")
	require.NoError(t, err)
}

func TestRenameFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := NewMockgcsClient(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := &Config{BucketName: "test-bucket"}

	fs := &FileSystem{
		conn:    mockConn,
		config:  config,
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	tests := []struct {
		name          string
		initialName   string
		newName       string
		setupMocks    func()
		expectedError bool
	}{
		{
			name:        "Rename file to new name",
			initialName: "dir/file.txt",
			newName:     "dir/file-renamed.txt",
			setupMocks: func() {
				mockConn.EXPECT().CopyObject(gomock.Any(), "dir/file.txt", "dir/file-renamed.txt").Return(nil)
				mockConn.EXPECT().DeleteObject(gomock.Any(), "dir/file.txt").Return(nil)
			},
			expectedError: false,
		},
		{
			name:        "Rename file with copy failure",
			initialName: "dir/file.txt",
			newName:     "dir/file-renamed.txt",
			setupMocks: func() {
				mockConn.EXPECT().CopyObject(gomock.Any(), "dir/file.txt", "dir/file-renamed.txt").Return(errors.New("copy failed"))
			},
			expectedError: true,
		},
		{
			name:        "Rename file with delete failure",
			initialName: "dir/file.txt",
			newName:     "dir/file-renamed.txt",
			setupMocks: func() {
				mockConn.EXPECT().CopyObject(gomock.Any(), "dir/file.txt", "dir/file-renamed.txt").Return(nil)
				mockConn.EXPECT().DeleteObject(gomock.Any(), "dir/file.txt").Return(errors.New("delete failed"))
			},
			expectedError: true,
		},
		{
			name:          "Rename file to same name",
			initialName:   "dir/file.txt",
			newName:       "dir/file.txt",
			setupMocks:    func() {}, // No calls expected
			expectedError: false,
		},
		{
			name:          "Rename file to different directory (not allowed)",
			initialName:   "dir1/file.txt",
			newName:       "dir2/file.txt",
			setupMocks:    func() {}, // No calls expected
			expectedError: true,
		},
	}

	// Set up logger mocks globally
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			err := fs.Rename(tt.initialName, tt.newName)

			if tt.expectedError {
				require.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err, "Unexpected error: %v", err)
			}
		})
	}
}

// func Test_OpenFile_GCS(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockGCS := NewMockgcsClient(ctrl)
// 	mockLogger := NewMockLogger(ctrl)
// 	mockMetrics := NewMockMetrics(ctrl)

// 	config := &Config{
// 		BucketName:      "test-bucket",
// 		CredentialsJSON: "fake-creds",
// 		ProjectID:       "test-project",
// 	}

// 	fs := &FileSystem{
// 		conn:    mockGCS,
// 		logger:  mockLogger,
// 		config:  config,
// 		metrics: mockMetrics,
// 	}

// 	expectedContent := "Hello, GCS!"

// 	// 👇 Create a mocked reader that returns your expected content
// 	mockReader := &storage.Reader{
// 		reader: io.NopCloser(strings.NewReader("dummy data")), // or fakeReader
// 	}

// 	// Set up expectations
// 	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
// 	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
// 	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

// 	mockGCS.EXPECT().
// 		NewReader(gomock.Any(), "abc/a1.txt").
// 		Return(mockReader, nil)

// 	mockGCS.EXPECT().
// 		StatObject(gomock.Any(), "abc/a1.txt").
// 		Return(&storage.ObjectAttrs{}, nil)

// 	// Act
// 	file, err := fs.OpenFile("abc/a1.txt", 0, os.ModePerm)
// 	require.NoError(t, err, "Failed to open file")

// 	content := make([]byte, 200)
// 	n, err := file.Read(content)
// 	require.NoError(t, err, "Failed to read file content")

// 	require.Equal(t, expectedContent, string(content[:n]), "File content does not match")
// }

// func Test_OpenFile_GCS(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	mockGCS := NewMockgcsClient(ctrl)
// 	mockLogger := NewMockLogger(ctrl)
// 	mockMetrics := NewMockMetrics(ctrl)

// 	config := &Config{
// 		BucketName:      "test-bucket",
// 		CredentialsJSON: "fake-creds",
// 		ProjectID:       "test-project",
// 	}

// 	fs := &FileSystem{
// 		conn:    mockGCS,
// 		logger:  mockLogger,
// 		config:  config,
// 		metrics: mockMetrics,
// 	}

// 	expectedContent := "Hello, GCS!"
// 	mockReader := ioutil.NopCloser(strings.NewReader(expectedContent))

// 	// Expect logger calls (optional but commonly included)
// 	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
// 	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
// 	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

// 	// Set up mock for GCS client
// 	mockGCS.EXPECT().
// 		NewReader(gomock.Any(), "abc/a1.txt").
// 		Return(mockReader, nil)

// 	mockGCS.EXPECT().
// 		StatObject(gomock.Any(), "abc/a1.txt").
// 		Return(&storage.ObjectAttrs{}, nil)

// 	// Act
// 	file, err := fs.OpenFile("abc/a1.txt", 0, os.ModePerm)
// 	require.NoError(t, err, "Failed to open file")

// 	content := make([]byte, 200)
// 	n, err := file.Read(content)
// 	require.NoError(t, err, "Failed to read file content")

// 	require.Equal(t, expectedContent, string(content[:n]), "File content does not match")
// }
