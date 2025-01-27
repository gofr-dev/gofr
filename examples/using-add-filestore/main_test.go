package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/cmd"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/testutil"
)

type mockFileInfo struct {
	name string
}

func (m mockFileInfo) Name() string     { return m.name }
func (mockFileInfo) Size() int64        { return 0 }
func (mockFileInfo) Mode() os.FileMode  { return 0 }
func (mockFileInfo) ModTime() time.Time { return time.Now() }
func (mockFileInfo) IsDir() bool        { return false }
func (mockFileInfo) Sys() any           { return nil }

func getContext(request gofr.Request, fileMock file.FileSystem) *gofr.Context {
	return &gofr.Context{
		Context:   context.Background(),
		Request:   request,
		Container: &container.Container{File: fileMock},
	}
}

func TestPwdCommandHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := file.NewMockFileSystemProvider(ctrl)

	mock.EXPECT().Getwd().DoAndReturn(func() (string, error) {
		return "/", nil
	})

	ctx := getContext(nil, mock)

	workingDirectory, _ := pwdCommandHandler(ctx)

	assert.Contains(t, workingDirectory, "/", "Test failed")
}

func TestLSCommandHandler(t *testing.T) {
	var (
		res any
		err error
	)

	path := "/"

	logs := testutil.StdoutOutputForFunc(func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().ReadDir(path).DoAndReturn(func(_ string) ([]file.FileInfo, error) {
			files := []file.FileInfo{
				mockFileInfo{name: "file1.txt"},
				mockFileInfo{name: "file2.txt"},
			}

			return files, nil
		})

		r := cmd.NewRequest([]string{"command", "ls", "-path=/"})

		ctx := getContext(r, mock)

		res, err = lsCommandHandler(ctx)
	})

	require.NoError(t, err)
	assert.Equal(t, "", res)
	assert.Contains(t, logs, "file1.txt", "Test failed")
	assert.Contains(t, logs, "file2.txt", "Test failed")
	assert.NotContains(t, logs, "file3.txt", "Test failed")
}

func TestGrepCommandHandler(t *testing.T) {
	var (
		res any
		err error
	)

	path := "/"

	logs := testutil.StdoutOutputForFunc(func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().ReadDir("/").DoAndReturn(func(_ string) ([]file.FileInfo, error) {
			files := []file.FileInfo{
				mockFileInfo{name: "file1.txt"},
				mockFileInfo{name: "file2.txt"},
			}

			return files, nil
		})

		r := cmd.NewRequest([]string{"command", "grep", "-keyword=fi", fmt.Sprintf("-path=%s", path)})
		ctx := getContext(r, mock)

		res, err = grepCommandHandler(ctx)
	})

	require.NoError(t, err)
	assert.Equal(t, "", res)
	assert.Contains(t, logs, "file1.txt", "Test failed")
	assert.Contains(t, logs, "file2.txt", "Test failed")
	assert.NotContains(t, logs, "file3.txt", "Test failed")
}

func TestCreateFileCommand(t *testing.T) {
	fileName := "file.txt"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := file.NewMockFileSystemProvider(ctrl)

	mock.EXPECT().Create(fileName).DoAndReturn(func(_ string) (file.File, error) {
		return &file.MockFile{}, nil
	})

	r := cmd.NewRequest([]string{"command", "createfile", fmt.Sprintf("-filename=%s", fileName)})
	ctx := getContext(r, mock)

	output, _ := createFileCommandHandler(ctx)

	assert.Contains(t, output, "Successfully created file: file.txt", "Test failed")
}

func TestRmCommand(t *testing.T) {
	fileName := "file.txt"
	os.Args = []string{"command", "rm", fmt.Sprintf("-filename=%s", fileName)}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := file.NewMockFileSystemProvider(ctrl)

	mock.EXPECT().Remove("file.txt").DoAndReturn(func(_ string) error {
		return nil
	})

	r := cmd.NewRequest([]string{"command", "rm", fmt.Sprintf("-filename=%s", fileName)})

	ctx := getContext(r, mock)

	output, _ := rmCommandHandler(ctx)

	assert.Contains(t, output, "Successfully removed file: file.txt", "Test failed")
}
