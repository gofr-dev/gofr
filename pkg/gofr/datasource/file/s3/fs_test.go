package s3

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	file "gofr.dev/pkg/gofr/datasource/file"
)

func Test_CreateRemoveFile(t *testing.T) {
	type testCase struct {
		name        string
		createPath  string
		removePath  string
		expectError bool
		removeAll   bool
	}

	tests := []testCase{
		{name: "Create and remove txt file", createPath: "abc.txt", removePath: "abc.txt", expectError: false, removeAll: false},
		{name: "Create and remove png file", createPath: "abc.png", removePath: "abc.png", expectError: false, removeAll: false},
		{name: "Create and remove jpeg file", createPath: "abc.jpeg", removePath: "abc.jpeg", expectError: false, removeAll: false},
		{name: "Create and remove json file", createPath: "abc.json", removePath: "abc.json", expectError: false, removeAll: false},
		{name: "Create and remove html file", createPath: "abc.html", removePath: "abc.html", expectError: false, removeAll: false},
		{name: "Create and remove octet-stream file", createPath: "abc", removePath: "abc", expectError: false, removeAll: false},
		{name: "Create file with invalid path", createPath: "abc/abc.txt", removePath: "", expectError: true, removeAll: false},
		{name: "Create and remove file in directory", createPath: "abc/efg.txt", removePath: "abc", expectError: false, removeAll: true},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runS3Test(t, func(fs file.FileSystemProvider) {
				if tt.removeAll {
					err := fs.Mkdir("abc", os.ModePerm)
					require.NoError(t, err, "TEST[%d] Failed. Desc %v", i, "Failed to create directory")
				}

				_, err := fs.Create(tt.createPath)
				if tt.expectError {
					require.Error(t, err, "TEST[%d] Failed. Desc %v", i, "Expected error during file creation")
					return
				}

				require.NoError(t, err, "TEST[%d] Failed. Desc %v", i, "Failed to create file")

				if tt.removePath != "" {
					err = fs.Remove(tt.removePath)
					require.NoError(t, err, "TEST[%d] Failed. Desc %v", i, "Failed to remove file")
				}

				if tt.removeAll {
					err = fs.RemoveAll("abc")
					require.NoError(t, err, "TEST[%d] Failed. Desc: %v", i, "Failed to remove directory")
				}
			})
		})
	}
}

func Test_OpenFile(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		_, err := fs.Create("abc.json")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to create file")

		_, err = fs.OpenFile("abc.json", 0, os.ModePerm)
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to open file")

		err = fs.Remove("abc.json")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to remove file")
	})
}

func Test_MakingAndDeletingDirectories(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		err := fs.MkdirAll("abc/bcd/cfg", os.ModePerm)
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error creating directory")

		_, err = fs.Create("abc/bcd/cfg/file.txt")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error creating file")

		err = fs.RemoveAll("abc")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error removing abc directory")
	})
}

func Test_RenameFile(t *testing.T) {
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

	runS3Test(t, func(fs file.FileSystemProvider) {
		_, err := fs.Create("abcd.json")
		require.NoError(t, err, "Failed to create initial file")

		// Iterate through each test case
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err = fs.Rename(tt.initialName, tt.newName)

				if tt.expectedError {
					require.Error(t, err, "Expected error but got none for case: %s", tt.name)
				} else {
					require.NoError(t, err, "Unexpected error for case: %s", tt.name)
				}
			})
		}

		err = fs.Remove("abc.json")
		require.NoError(t, err, "Failed to remove file")
	})
}

func Test_RenameDirectory(t *testing.T) {
	runS3Test(t, func(fs file.FileSystemProvider) {
		err := fs.Mkdir("abc/bcd/cfg", os.ModePerm)
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v:", 0, "Failed to create directory")

		_, err = fs.Create("abc/bcd/cfg/file.txt")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error creating file")

		err = fs.Rename("abc", "abcd")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to rename directory")

		err = fs.RemoveAll("abcd")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to remove directory")
	})
}

func Test_ReadDir(t *testing.T) {
	type result struct {
		name  string
		size  int64
		isDir bool
	}

	tests := []struct {
		dirPath         string
		expectedResults []result
	}{
		{
			dirPath: "abc/efg",
			expectedResults: []result{
				{"file.txt", 0, false},
				{"hij", 0, true},
			},
		},
		{
			dirPath: "abc",
			expectedResults: []result{
				{"efg", 0, true},
			},
		},
		{
			dirPath: ".",
			expectedResults: []result{
				{"abc", 0, true},
			},
		},
	}

	runS3Test(t, func(fs file.FileSystemProvider) {
		// Setup
		currentDir, err := fs.Getwd()
		require.NoError(t, err, "Failed to get current directory")
		assert.Equal(t, "/gofr-bucket-2", currentDir)

		err = fs.Mkdir("abc/efg/hij", os.ModePerm)
		require.NoError(t, err, "Error creating directory")

		_, err = fs.Create("abc/efg/file.txt")
		require.NoError(t, err, "Error creating file")

		for i, tt := range tests {
			t.Run(fmt.Sprintf("TestCase %d", i), func(t *testing.T) {
				res, err := fs.ReadDir(tt.dirPath)
				require.NoError(t, err, "Error reading directory")

				var results []result

				for _, entry := range res {
					results = append(results, result{entry.Name(), entry.Size(), entry.IsDir()})
				}

				assert.Equal(t, tt.expectedResults, results, "Mismatch in results for path: %v", tt.dirPath)
			})
		}

		err = fs.RemoveAll("abc")
		require.NoError(t, err, "Error removing directory")
	})
}

func Test_StatFile(t *testing.T) {
	type result struct {
		name  string
		size  int64
		isDir bool
	}

	expectedResponse := result{
		name:  "file.txt",
		size:  0,
		isDir: false,
	}

	runS3Test(t, func(fs file.FileSystemProvider) {
		err := fs.Mkdir("dir1/dir2", os.ModePerm)
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to create directory")

		_, err = fs.Create("dir1/dir2/file.txt")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to create file")

		res, err := fs.Stat("dir1/dir2/file.txt")

		response := result{res.Name(), res.Size(), res.IsDir()}

		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error getting file info")
		assert.Equal(t, expectedResponse, response, "Mismatch in results for path: %v", expectedResponse.name)

		err = fs.RemoveAll("dir1")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error removing directory")
	})
}

func Test_StatDirectory(t *testing.T) {
	type result struct {
		name  string
		size  int64
		isDir bool
	}

	expectedResponse := result{
		name:  "dir2",
		size:  0,
		isDir: false,
	}

	runS3Test(t, func(fs file.FileSystemProvider) {
		err := fs.Mkdir("dir1/dir2", os.ModePerm)
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Failed to create directory")

		_, err = fs.Create("dir1/dir2/file.txt")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error creating file")

		res, err := fs.Stat("dir1/dir2")

		response := result{res.Name(), res.Size(), res.IsDir()}

		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error getting directory stats")
		assert.Equal(t, expectedResponse, response, "Mismatch in results for path: %v", expectedResponse.name)

		err = fs.RemoveAll("dir1")
		require.NoError(t, err, "TEST[%d] Failed. Desc: %v", 0, "Error removing directory")
	})
}

// Helper functions.
func createBucket(t *testing.T, fs file.FileSystemProvider) {
	t.Helper()

	f, ok := fs.(*fileSystem)
	require.True(t, ok)

	_, err := f.conn.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String("gofr-bucket-2"),
	})
	require.NoError(t, err)
}

func deleteBucket(t *testing.T, fs file.FileSystemProvider) {
	t.Helper()

	f, ok := fs.(*fileSystem)
	require.True(t, ok)

	_, err := f.conn.DeleteBucket(context.TODO(), &s3.DeleteBucketInput{
		Bucket: aws.String("gofr-bucket-2"),
	})
	require.NoError(t, err)
}

func runS3Test(t *testing.T, testFunc func(fs file.FileSystemProvider)) {
	t.Helper()

	cfg := Config{
		"http://localhost:4566",
		"gofr-bucket-2",
		"us-east-1",
		"test",
		"test",
	}

	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	s3Client := New(&cfg)

	s3Client.UseLogger(mockLogger)
	s3Client.UseMetrics(mockMetrics)

	s3Client.Connect()

	createBucket(t, s3Client)
	testFunc(s3Client)
	deleteBucket(t, s3Client)
}
