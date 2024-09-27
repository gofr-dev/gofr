package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/testutil"
)

type mockFileInfo struct {
	name string
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return 0 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() interface{}   { return nil }

func TestPwdCommand(t *testing.T) {
	os.Args = []string{"command", "pwd"}
	logs := testutil.StdoutOutputForFunc(func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app := gofr.NewCMD()
		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()
		mock.EXPECT().Getwd().DoAndReturn(func() (string, error) {
			return "/", nil
		})
		app.AddFileStore(mock)
		registerPwdCommand(app, mock)
		app.Run()
	})
	assert.Contains(t, logs, "/", "Test failed")
}

func TestLSCommand(t *testing.T) {
	path := "/"
	os.Args = []string{"command", "ls", fmt.Sprintf("-path=%s", path)}
	logs := testutil.StdoutOutputForFunc(func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app := gofr.NewCMD()
		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()
		mock.EXPECT().ReadDir(path).DoAndReturn(func(s string) ([]file.FileInfo, error) {
			var files []file.FileInfo = []file.FileInfo{
				mockFileInfo{name: "file1.txt"},
				mockFileInfo{name: "file2.txt"},
			}
			return files, nil
		})
		app.AddFileStore(mock)
		registerLsCommand(app, mock)
		app.Run()
	})
	assert.Contains(t, logs, "file1.txt", "Test failed")
	assert.Contains(t, logs, "file2.txt", "Test failed")
	assert.NotContains(t, logs, "file3.txt", "Test failed")
}

func TestGrepCommand(t *testing.T) {
	path := "/"
	os.Args = []string{"command", "grep", "-keyword=fi", fmt.Sprintf("-path=%s", path)}
	logs := testutil.StdoutOutputForFunc(func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app := gofr.NewCMD()
		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()
		mock.EXPECT().ReadDir("/").DoAndReturn(func(s string) ([]file.FileInfo, error) {
			var files []file.FileInfo = []file.FileInfo{
				mockFileInfo{name: "file1.txt"},
				mockFileInfo{name: "file2.txt"},
			}
			return files, nil
		})
		app.AddFileStore(mock)
		registerGrepCommand(app, mock)
		app.Run()
	})
	assert.Contains(t, logs, "file1.txt", "Test failed")
	assert.Contains(t, logs, "file2.txt", "Test failed")
	assert.NotContains(t, logs, "file3.txt", "Test failed")
}

func TestCreateFileCommand(t *testing.T) {
	fileName := "file.txt"
	os.Args = []string{"command", "createfile", fmt.Sprintf("-filename=%s", fileName)}
	logs := testutil.StdoutOutputForFunc(func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app := gofr.NewCMD()
		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()
		mock.EXPECT().Create(fileName).DoAndReturn(func(s string) (file.File, error) {
			return &file.MockFile{}, nil
		})
		app.AddFileStore(mock)
		registerCreateFileCommand(app, mock)
		app.Run()
	})
	assert.Contains(t, logs, "Creating file :file.txt", "Test failed")
	assert.Contains(t, logs, "Succesfully created file:file.txt", "Test failed")
}

func TestRmCommand(t *testing.T) {
	fileName := "file.txt"
	os.Args = []string{"command", "rm", fmt.Sprintf("-filename=%s", fileName)}
	logs := testutil.StdoutOutputForFunc(func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app := gofr.NewCMD()
		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()
		mock.EXPECT().Remove("file.txt").DoAndReturn(func(filename string) error {
			return nil
		})
		app.AddFileStore(mock)
		registerRmCommand(app, mock)
		app.Run()
	})
	assert.Contains(t, logs, "Removing file :file.txt", "Test failed")
	assert.Contains(t, logs, "Succesfully removed file:file.txt", "Test failed")
}
