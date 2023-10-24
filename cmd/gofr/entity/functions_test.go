package entity

import (
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/cmd/gofr/migration"
)

func Test_populateEntityFile(t *testing.T) {
	dir := t.TempDir()

	path, err := os.MkdirTemp(dir, "testProject")
	if err != nil {
		t.Errorf("Received unexpected error:\n%+v", err)

		return
	}

	defer os.RemoveAll(path)

	err = os.Chdir(path)
	if err != nil {
		t.Errorf("Error while changing directory:\n%+v", err)
	}

	testFile, _ := os.OpenFile("test.go", os.O_CREATE|os.O_RDONLY, migration.RWMode)
	mainFile, _ := os.OpenFile("main.go", os.O_RDONLY, migration.RWMode)

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)

	type args struct {
		entity string
		types  string
	}

	tests := []struct {
		name      string
		args      args
		mockCalls []*gomock.Call
		wantErr   bool
	}{
		{"error: Chdir", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(errors.New("test error")).Times(1),
		}, true},

		{"error: OpenFile", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1),
		}, true},

		{"error: OpenFile returns nil", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
		}, true},

		{"error: OpenFile returns nil", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(testFile, nil).Times(1),
		}, true},

		{"error: OpenFile returns nil", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(mainFile, nil).Times(1),
		}, true},
	}

	for _, tt := range tests {
		if err := populateEntityFile(c, dir, path, tt.args.entity, tt.args.types); (err != nil) != tt.wantErr {
			t.Errorf("Test %v: populateEntityFile() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func Test_createModel(t *testing.T) {
	path, err := os.MkdirTemp("", "testProject")
	if err != nil {
		t.Errorf("Received unexpected error:\n%+v", err)

		return
	}

	defer os.RemoveAll(path)

	err = os.Chdir(path)
	if err != nil {
		t.Errorf("Error while changing directory:\n%+v", err)
	}

	testFile, _ := os.OpenFile("testRead.go", os.O_CREATE|os.O_RDONLY, migration.RWMode)

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)

	tests := []struct {
		name      string
		entity    string
		mockCalls []*gomock.Call
		wantErr   bool
	}{
		{"error case: chdir", "brand", []*gomock.Call{
			c.EXPECT().Stat(gomock.Any()).Return(nil, nil).AnyTimes(),
			c.EXPECT().IsNotExist(gomock.Any()).Return(false).AnyTimes(),
			c.EXPECT().Chdir(gomock.Any()).Return(errors.New("test error")).Times(1),
		}, true},

		{"error case: openfile", "brand", []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1),
		}, true},

		{"error case: openfile returns nil", "brand", []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
		}, true},

		{"error case: openfile returns nil", "brand", []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(testFile, nil).Times(1),
		}, true},
	}
	for _, tt := range tests {
		if err := createModel(c, path, tt.entity); (err != nil) != tt.wantErr {
			t.Errorf("Test %v: createModel() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func Test_populateInterfaceFiles(t *testing.T) {
	dir := t.TempDir()

	path, err := os.MkdirTemp(dir, "testProject")
	if err != nil {
		t.Errorf("Received unexpected error:\n%+v", err)

		return
	}

	defer os.RemoveAll(path)

	_ = os.Chdir(path)

	testFile, _ := os.OpenFile("test.go", os.O_CREATE|os.O_RDONLY, migration.RWMode)

	err = populateInterfaceFiles("test", dir, "core", testFile)

	assert.Error(t, err)
}
