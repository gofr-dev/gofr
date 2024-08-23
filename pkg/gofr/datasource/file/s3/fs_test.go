package s3

import (
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

func TestDeleteBucket(t *testing.T) {

}

func Test_CreateFile(t *testing.T) {
	//runS3Test(t, func(fs file_interface.FileSystemProvider) {
	//	_, err := fs.Create("abc.txt")
	//	if err != nil {
	//		t.Error(err)
	//	}
	//
	//})
	//runS3Test(t, func(fs file_interface.FileSystemProvider) {
	//	_, err := fs.Create("abc.png")
	//	if err != nil {
	//		t.Error(err)
	//	}
	//
	//})
	//runS3Test(t, func(fs file_interface.FileSystemProvider) {
	//	_, err := fs.Create("abc.jpeg")
	//	if err != nil {
	//		t.Error(err)
	//	}
	//
	//})
	//runS3Test(t, func(fs file_interface.FileSystemProvider) {
	//	_, err := fs.Create("abc.json")
	//	if err != nil {
	//		t.Error(err)
	//	}
	//
	//})
	//runS3Test(t, func(fs file_interface.FileSystemProvider) {
	//	_, err := fs.Create("abc.html")
	//	if err != nil {
	//		t.Error(err)
	//	}
	//
	//})
	//runS3Test(t, func(fs file_interface.FileSystemProvider) {
	//	_, err := fs.Create("abc") // octet-stream
	//	if err != nil {
	//		t.Error(err)
	//	}
	//
	//})
	//runS3Test(t, func(fs file_interface.FileSystemProvider) {
	//	_, err := fs.Create("abc/abc.txt") // octet-stream
	//	if err != nil {
	//		t.Error(err)
	//	}
	//
	//})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		_, err := fs.Create("abc/bcd/abc.txt") // octet-stream
		if err != nil {
			t.Error(err)
		}

	})
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Chdir("abc/bcd")
		_, err := fs.Create("abc/bcd/abc.txt") // octet-stream
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

func Test_MkDir(t *testing.T) {
	runS3Test(t, func(fs file_interface.FileSystemProvider) {
		err := fs.Mkdir("abc/bcd", os.ModePerm)
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
