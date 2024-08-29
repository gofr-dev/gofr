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

	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

// Creating different file formats and removing them
func Test_CreateRemoveFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.txt")
		require.NoError(t, err)

		err = fs.Remove("abc.txt")
		require.NoError(t, err)
	})

	runS3Test(t, func(fs file_interface.FileSystemProvider) {

		_, err := fs.Create("abc.png")
		require.NoError(t, err)

		err = fs.Remove("abc.png")
		require.NoError(t, err)

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.jpeg")
		require.NoError(t, err)

		err = fs.Remove("abc.jpeg")
		require.NoError(t, err)

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.json")
		require.NoError(t, err)

		err = fs.Remove("abc.json")
		require.NoError(t, err)

	})

	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.html")
		require.NoError(t, err)

		err = fs.Remove("abc.html")
		require.NoError(t, err)
	})

	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc") // octet-stream
		require.NoError(t, err)

		// remove considers path with no extension to be of file format "application/octet-stream"
		err = fs.Remove("abc")
		require.NoError(t, err)
	})

	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc/abc.txt")
		require.Error(t, err)
	})

	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Mkdir("abc", os.ModePerm)
		require.NoError(t, err)

		_, err = fs.Create("abc/efg.txt") // text file
		require.NoError(t, err)

		err = fs.RemoveAll("abc")
		require.NoError(t, err)
	})
}

func Test_OpenFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.json")
		require.NoError(t, err)

		_, err = fs.OpenFile("abc.json", 0, os.ModePerm)
		require.NoError(t, err)

		err = fs.Remove("abc.json")
	})
}

func Test_MakingAndDeletingDirectories(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.MkdirAll("abc/bcd/cfg", os.ModePerm)
		require.NoError(t, err, "error creating directory")

		_, err = fs.Create("abc/bcd/cfg/file.txt")
		require.NoError(t, err, "error creating file")

		err = fs.RemoveAll("abc")
		require.NoError(t, err, "error removing abc directory")
	})
}

func Test_RenameFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abcd.json")
		require.NoError(t, err)

		err = fs.Rename("abcd.json", "abc.json")
		require.NoError(t, err)

		err = fs.Remove("abc.json")
		require.NoError(t, err)
	})
}

func Test_RenameDirectory(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Mkdir("abc/bcd/cfg", os.ModePerm)
		require.NoError(t, err)

		_, err = fs.Create("abc/bcd/cfg/file.txt")
		require.NoError(t, err, "error creating file")

		err = fs.Rename("abc", "abcd")
		require.NoError(t, err)

		err = fs.RemoveAll("abcd")
		require.NoError(t, err)
	})
}

// works
func Test_ReadDir(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		fs.ChDir("gofr-bucket-2")
		currentDir, err := fs.Getwd()
		require.NoError(t, err)
		assert.Equal(t, "/gofr-bucket-2", currentDir)

		err = fs.Mkdir("abc/efg/hij", os.ModePerm)
		require.NoError(t, err)

		_, err = fs.Create("abc/efg/file.txt")

		res, err := fs.ReadDir("abc/efg")
		require.NoError(t, err)

		for i := range res {
			fmt.Println(res[i].Name(), res[i].Size(), res[i].IsDir())
		}

		err = fs.RemoveAll("abc")
		require.NoError(t, err)

	})
}

func Test_StatFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Mkdir("dir1/dir2", os.ModePerm)
		require.NoError(t, err)

		_, err = fs.Create("dir1/dir2/file.txt")

		res, err := fs.Stat("dir1/dir2/file.txt")
		require.NoError(t, err)
		fmt.Println(res.Name(), res.Size(), res.IsDir())

		err = fs.RemoveAll("dir1")
	})
}

func Test_StatDirectory(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Mkdir("dir1/dir2", os.ModePerm)
		require.NoError(t, err)

		_, err = fs.Create("dir1/dir2/file.txt")

		res, err := fs.Stat("dir1/dir2")
		require.NoError(t, err)
		fmt.Println(res.Name(), res.Size(), res.IsDir())

		err = fs.RemoveAll("dir1")
	})
}

func createBucket(t *testing.T, fs file_interface.FileSystemProvider) {
	t.Helper()
	f, ok := fs.(*fileSystem)
	require.True(t, ok)

	_, err := f.conn.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String("gofr-bucket-2"),
	})
	require.NoError(t, err)

}

func deleteBucket(t *testing.T, fs file_interface.FileSystemProvider) {
	t.Helper()

	f, ok := fs.(*fileSystem)
	require.True(t, ok)

	_, err := f.conn.DeleteBucket(context.TODO(), &s3.DeleteBucketInput{
		Bucket: aws.String("gofr-bucket-2"),
	})
	require.NoError(t, err)
}

func runS3Test(t *testing.T, testFunc func(fs file_interface.FileSystemProvider)) {
	t.Helper()

	cfg := Config{
		// currently using localstack in the tests, to use aws change to "https://s3.amazonaws.com" - aws base endpoint
		"http://localhost:4566",
		"gofr-bucket-2",
		"us-east-1",
		// localstack does not check credentials. However, we need to pass correct access keys in case we are using AWS.
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

	// Run the test function with the initialized file system
	createBucket(t, s3Client)
	testFunc(s3Client)
	deleteBucket(t, s3Client)
}
