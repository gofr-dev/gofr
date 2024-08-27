package s3

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	file_interface "gofr.dev/pkg/gofr/datasource/file"
	"os"
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

func Test_CreateFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.txt")
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.png")
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.jpeg")
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.json")
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc.html")
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc") // octet-stream
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc/abc.txt") // octet-stream
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc/bcd/abc.txt") // text file
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.ChDir("abc/bcd")
		_, err = fs.Create("efg.txt") // text file
		if err != nil {
			t.Error(err)
		}

	})

}

func Test_RemoveFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Remove("abc")
		if err != nil {
			t.Error(err)
		}

		currentDir, err := fs.Getwd()
		if err != nil {
			t.Error(err)
		}
		fmt.Println("current dir:", currentDir)

	})

	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Remove("abc/abc.txt")
		if err != nil {
			t.Error(err)
		}

	})

	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Remove("abc.json")
		if err != nil {
			t.Error(err)
		}

	})
}

func Test_RemoveAll(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.RemoveAll("abc") // octet-stream
		if err != nil {
			t.Error(err)
		}

	})
}

func Test_RenameFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Rename("abc.json", "abcd.json")
		if err != nil {
			t.Error(err)
		}
	})
}

func Test_OpenFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.OpenFile("abcd.json", 0, os.ModePerm)
		if err != nil {
			t.Error(err)
		}

	})
}

// TODO: We can add permission only while making bucket and not inside directories....
func Test_MkDir(t *testing.T) {
	//runS3Test(t, func(fs file_interface.FileSystemProvider) {
	//	fs.ChDir("gofr-bucket-2")
	//	err := fs.Mkdir("abc/cfg", os.ModePerm)
	//	if err != nil {
	//		t.Error(err)
	//	}
	//})

	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Mkdir("abc/cfg", os.ModePerm)
		if err != nil {
			t.Error(err)
		}
	})
}

func Test_ReadDir(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		fs.ChDir("gofr-bucket-2")
		fs.ChDir("abc")
		currentDir, _ := fs.Getwd()
		assert.Equal(t, "/gofr-bucket-2/abc", currentDir)

		res, err := fs.ReadDir("bcd")
		require.NoError(t, err)
		for i := range res {
			fmt.Println(res[i].Name(), res[i].Size(), res[i].IsDir())
		}

	})
}

func Test_StatFile(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		fs.ChDir("gofr-bucket-2")
		res, err := fs.Stat("abc/bcd/efg.txt")
		require.NoError(t, err)
		fmt.Println(res.Name(), res.Size(), res.IsDir())
	})
}

func Test_StatDirectory(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		fs.ChDir("gofr-bucket-2")
		res, err := fs.Stat("abc")
		require.NoError(t, err)
		fmt.Println(res.Name(), res.Size(), res.IsDir())
	})
}

func runS3Test(t *testing.T, testFunc func(fs file_interface.FileSystemProvider)) {
	t.Helper()

	cfg := Config{
		"http://localhost:4566",
		"user",
		"",
		"us-east-1",
		"general-purpose",
		"AKIAYHJANQGSVIE2CX7F",
		"ZQaoxNLYiIcdHMwGJJwhPp7ksyyjW27q4eLFTYxZ",
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
	testFunc(s3Client)
}
