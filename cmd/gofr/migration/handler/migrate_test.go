package handler

import (
	"bufio"
	"fmt"
	"go/build"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func Test_MigrateValidationFail(t *testing.T) {
	testCases := []struct {
		desc   string
		params map[string]string
		expErr error
	}{
		{"failure case: invalid params", map[string]string{"invalid": "value", "method": "test", "database": "testDB"},
			&errors.Response{Reason: "unknown parameter(s) [invalid]. Run gofr <command_name> -h for help of the command."}},
		{"failure case: missing Params", map[string]string{"database": "testDB"}, errors.MissingParam{Param: []string{"method"}}},
	}

	for i, tc := range testCases {
		req := httptest.NewRequest("", setQueryParams(tc.params), http.NoBody)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		res, err := Migrate(ctx)

		assert.Nil(t, res)
		assert.EqualErrorf(t, tc.expErr, err.Error(), "Test[%d] Failed", i)
	}
}

func TestHandler_Mkdir(t *testing.T) {
	const testDir = "test-dir"

	defer os.RemoveAll(testDir)

	tests := []struct {
		desc string
		name string
		err  error
	}{
		{"success case: creating directory", testDir, nil},
		{"error case: creating empty directory", "", &fs.PathError{}},
	}

	for i, tc := range tests {
		var s Handler

		err := s.Mkdir(tc.name, fs.FileMode(0777))

		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestHandler_Getwd(t *testing.T) {
	var h Handler

	dir, _ := os.Getwd()

	_, err := h.Getwd()

	assert.Equal(t, "handler", path.Base(dir))

	assert.Nil(t, err, "Test case failed")
}

func Test_getModulePath(t *testing.T) {
	dir := t.TempDir()
	modFilePath := path.Join(dir, "go.mod")

	f, err := os.Create(modFilePath)
	if err != nil {
		t.Errorf("error in creating file: %v", err)
	}

	err = os.WriteFile(modFilePath, []byte("module example.com/my-project\n\ngo 1.17\n"), os.ModeDevice)
	if err != nil {
		t.Errorf("error in writing to mod file: %v", err)
	}

	ctrl := gomock.NewController(t)
	fsMigrate := NewMockFSMigrate(ctrl)

	fsMigrate.EXPECT().OpenFile("../go.mod", os.O_RDONLY, gomock.Any()).Return(f, nil)

	name, err := getModulePath(fsMigrate, "random-dir")

	assert.NoError(t, err)

	assert.Equal(t, "example.com/my-project", name)
}

func Test_createMain(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	mockFS := NewMockFSMigrate(ctrl)

	dir := t.TempDir()

	f, _ := os.Create(path.Join(dir, "test.txt"))
	f2, _ := os.Create(path.Join(dir, "main.go"))
	modFile, _ := os.Create(path.Join(dir, "go.mod"))

	_, _ = modFile.WriteString("module moduleName")
	defer modFile.Close()

	type args struct {
		method    string
		db        string
		directory string
	}

	var (
		filePath = "../go.mod"
		rwMode   = os.FileMode(0666)
	)

	tests := []struct {
		desc      string
		args      args
		mockCalls []*gomock.Call
		expErr    bool
	}{
		{"database not supported", args{"UP", "kafka", dir}, []*gomock.Call{}, true},
		{"Project Not in GOPATH error", args{"DOWN", "gorm", dir}, []*gomock.Call{
			mockFS.EXPECT().OpenFile(filePath, os.O_RDONLY, rwMode).Return(f, &errors.Response{Reason: "test error"}),
		}, true},
		{"success", args{"DOWN", "gorm", dir}, []*gomock.Call{
			mockFS.EXPECT().OpenFile(filePath, os.O_RDONLY, rwMode).Return(modFile, nil),
			mockFS.EXPECT().Stat("build").Return(nil, &errors.Response{Reason: "test error"}),
			mockFS.EXPECT().IsNotExist(&errors.Response{Reason: "test error"}).Return(true),
			mockFS.EXPECT().Mkdir("build", os.FileMode(0777)).Return(nil),
			mockFS.EXPECT().Chdir("build").Return(nil),
			mockFS.EXPECT().OpenFile("main.go", os.O_CREATE|os.O_WRONLY, rwMode).Return(f2, nil),
		}, false},
		{"mkdir error", args{"DOWN", "mongo", dir}, []*gomock.Call{
			mockFS.EXPECT().OpenFile(filePath, os.O_RDONLY, rwMode).Return(modFile, nil),
			mockFS.EXPECT().Stat("build").Return(nil, &errors.Response{Reason: "test error"}),
			mockFS.EXPECT().IsNotExist(&errors.Response{Reason: "test error"}).Return(true),
			mockFS.EXPECT().Mkdir("build", os.FileMode(0777)).Return(&errors.Response{Reason: "test error"}),
		}, true},
		{"chdir error", args{"DOWN", "cassandra", dir}, []*gomock.Call{
			mockFS.EXPECT().OpenFile(filePath, os.O_RDONLY, rwMode).Return(modFile, nil),
			mockFS.EXPECT().Stat("build").Return(nil, nil),
			mockFS.EXPECT().IsNotExist(nil).Return(false),
			mockFS.EXPECT().Chdir("build").Return(&errors.Response{Reason: "test error"}),
		}, true},
		{"openFile error", args{"DOWN", "redis", dir}, []*gomock.Call{
			mockFS.EXPECT().OpenFile(filePath, os.O_RDONLY, rwMode).Return(modFile, nil),
			mockFS.EXPECT().Stat("build").Return(nil, nil),
			mockFS.EXPECT().IsNotExist(nil).Return(false),
			mockFS.EXPECT().Chdir("build").Return(nil),
			mockFS.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &errors.Response{Reason: "test error"}),
		}, true},
		{"template execution error", args{"DOWN", "ycql", dir}, []*gomock.Call{
			mockFS.EXPECT().OpenFile(filePath, os.O_RDONLY, rwMode).Return(modFile, nil),
			mockFS.EXPECT().Stat("build").Return(nil, nil),
			mockFS.EXPECT().IsNotExist(nil).Return(false),
			mockFS.EXPECT().Chdir("build").Return(nil),
			mockFS.EXPECT().OpenFile("main.go", os.O_CREATE|os.O_WRONLY, rwMode).Return(nil, nil),
		}, true},
	}

	for i, tc := range tests {
		if err := createMain(mockFS, tc.args.method, tc.args.db, tc.args.directory, nil); (err != nil) != tc.expErr {
			t.Errorf("TEST[%d] failed: createMain(), Got: %v, Expected: %v", i, err, tc.expErr)
		}
	}
}

func Test_createMain_goPath_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer func() {
		ctrl.Finish()
	}()

	var (
		filePath = "../go.mod"
		rwMode   = os.FileMode(0666)
	)

	mockFS := NewMockFSMigrate(ctrl)
	dir := t.TempDir()

	t.Setenv("GOPATH", dir)

	build.Default.GOPATH = dir

	currDir, err := os.MkdirTemp(dir, "src")
	if err != nil {
		t.Errorf("received unexpected error:\n%+v", err)

		return
	}

	defer os.RemoveAll(currDir)

	dir += "/src"
	_ = os.Chdir(currDir)

	currDir, err = os.MkdirTemp(currDir, "gofr")
	if err != nil {
		t.Errorf("Received unexpected error:\n%+v", err)

		return
	}

	defer os.RemoveAll(currDir)

	dir += "/gofr"

	f, _ := os.CreateTemp("test.txt", currDir)
	f2, _ := os.Create("main.go")

	mockFS.EXPECT().OpenFile(filePath, os.O_RDONLY, rwMode).Return(f, &errors.Response{Reason: "test error"})
	mockFS.EXPECT().Stat("build").Return(nil, &errors.Response{Reason: "test error"})
	mockFS.EXPECT().IsNotExist(&errors.Response{Reason: "test error"}).Return(false)
	mockFS.EXPECT().Chdir("build").Return(nil)
	mockFS.EXPECT().OpenFile("main.go", os.O_CREATE|os.O_WRONLY, rwMode).Return(f2, nil)

	if err := createMain(mockFS, "DOWN", "GORM", dir, nil); err != nil {
		t.Errorf("FAILED: Success case GOPATH  : createMain() Got: %v, Expected %v", err, nil)
	}
}

func Test_runMigration(t *testing.T) {
	dir := t.TempDir()
	modFilePath := path.Join(dir, "go.mod")

	f, err := os.Create(modFilePath)
	if err != nil {
		t.Errorf("error in creating file: %v", err)
	}

	f1, _ := os.Create(path.Join(dir, "main.go"))

	if err != nil {
		t.Errorf("error in creating file: %v", err)
	}

	defer os.Remove("main.go")

	err = os.WriteFile(modFilePath, []byte("module example.com/my-project\n\ngo 1.17\n"), os.ModeDevice)
	if err != nil {
		t.Errorf("error in writing to mod file: %v", err)
	}

	ctrl := gomock.NewController(t)
	mockFS := NewMockFSMigrate(ctrl)

	type args struct {
		method string
		db     string
	}

	tests := []struct {
		name      string
		args      args
		mockCalls []*gomock.Call
		want      interface{}
		wantErr   bool
	}{
		{"Getwd() error", args{}, []*gomock.Call{
			mockFS.EXPECT().Getwd().Return("", &errors.Response{Reason: "test error"}).Times(1),
		}, nil, true},

		{"Chdir and  dir not exists error", args{}, []*gomock.Call{
			mockFS.EXPECT().Getwd().Return("", nil).MaxTimes(2),
			mockFS.EXPECT().Chdir("migrations").Return(nil).MaxTimes(2),
			mockFS.EXPECT().IsNotExist(nil).Return(true).Times(1),
		}, nil, true},

		{"createMain error", args{}, []*gomock.Call{
			mockFS.EXPECT().IsNotExist(gomock.Any()).Return(false),
			mockFS.EXPECT().Stat(gomock.Any()).Return(nil, nil).MaxTimes(1),
		}, nil, true},

		{"createMain cmd error", args{method: DOWN, db: "gorm"}, []*gomock.Call{
			mockFS.EXPECT().Getwd().Return("", nil),
			mockFS.EXPECT().Chdir("migrations").Return(nil).MaxTimes(1),
			mockFS.EXPECT().IsNotExist(gomock.Any()).Return(false).MaxTimes(2),
			mockFS.EXPECT().Stat(gomock.Any()).Return(nil, nil).MaxTimes(2),
			mockFS.EXPECT().Chdir("build").Return(nil).MaxTimes(1),
			mockFS.EXPECT().OpenFile("../go.mod", os.O_RDONLY, gomock.Any()).Return(f, nil).MaxTimes(1),
			mockFS.EXPECT().OpenFile("main.go", os.O_CREATE|os.O_WRONLY, gomock.Any()).Return(f1, nil).AnyTimes(),
		}, "", true},
	}

	for i, tt := range tests {
		got, err := runMigration(mockFS, tt.args.method, tt.args.db, nil)

		if (err != nil) != tt.wantErr {
			t.Errorf("TEST[%d] Failed runMigration() error = %v, wantErr %v", i, err, tt.wantErr)
			return
		}

		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("TEST[%d] Faile runMigration() got = %v, want %v", i, got, tt.want)
		}
	}
}

type mockFSMigrate struct {
	*MockFSMigrate
}

func (m mockFSMigrate) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// Test_importOrder tests if the imports are sorted in migration template
func Test_importOrder(t *testing.T) {
	dir := t.TempDir()

	err := os.Chdir(dir)
	if err != nil {
		t.Error(err)
	}

	ctrl := gomock.NewController(t)
	mockFS := mockFSMigrate{MockFSMigrate: NewMockFSMigrate(ctrl)}

	mockFS.EXPECT().Stat("build").Return(nil, nil)
	mockFS.EXPECT().IsNotExist(nil).Return(false)
	mockFS.EXPECT().Chdir("build").Return(nil)

	err = templateCreate(mockFS, "sample-api", "UP", "db := dbmigration.NewGorm(k.GORM())", "example.com/sample-api", nil)
	if err != nil {
		t.Errorf("expected no error, got:\n%v", err)
	}

	defer os.Remove("main.go")

	file, err := os.Open("main.go")
	if err != nil {
		t.Errorf("error in opening main.go file: %v", err)
	}

	defer func() {
		if err = file.Close(); err != nil {
			t.Logf("Error closing file: %s\n", err)
		}
	}()

	err = checkImportOrder(file)
	if err != nil {
		t.Errorf("expected no error, got:\n%v", err)
	}
}

// checkImportOrder returns error if the grouped imports are not sorted.

//nolint:gocognit // cannot be optimized without hampering the readability
func checkImportOrder(file *os.File) error {
	var (
		imports      = make([]string, 0)
		scanner      = bufio.NewScanner(file)
		appendImport = false
	)

	for scanner.Scan() {
		line := scanner.Text()

		if line == "import (" {
			appendImport = true
			continue
		}

		if appendImport {
			if line == "" || line == ")" {
				if !sort.StringsAreSorted(imports) {
					return errors.Error(fmt.Sprintf("unsorted imports in migration template\n%v", strings.Join(imports, "\n")))
				}

				imports = nil

				if line == ")" {
					break
				}

				continue
			}

			imports = append(imports, line)
		}
	}

	return nil
}

func TestMigrate_getDownString(t *testing.T) {
	input := []string{"build"}
	expOut := `map[string]dbmigration.Migrator{
"build": migrations.Kbuild{},
	}`

	output := getDownString(input)

	assert.Equal(t, expOut, output, "TEST failed.")
}

func TestMigrate_Help(t *testing.T) {
	var h Handler

	expOutHelp := `runs the migration for method UP or DOWN as provided and for the given database

usage: gofr migrate -method=<method> -database=<database>

Flag:
method: UP or DOWN
database: gorm  // gorm supports following dialects: mysql, mssql, postgres, sqlite

Examples:
gofr migrate -method=UP -database=gorm

`

	actual := h.Help()

	assert.Equal(t, expOutHelp, actual, "Test failed.")
}

func Test_MigrateError(t *testing.T) {
	var h Handler
	help := h.Help()

	tests := []struct {
		desc   string
		params map[string]string
		expOut interface{}
		expErr error
	}{
		{"using helper case", map[string]string{"h": "true"}, help, nil},
		{"migration do not exists error", map[string]string{"database": "db", "method": "UP"}, nil,
			&os.SyscallError{}},
		{"missing param database", map[string]string{"method": "test"},
			nil, errors.MissingParam{Param: []string{"database"}}},
		{"invalid method", map[string]string{"database": "testDB"},
			nil, errors.MissingParam{Param: []string{"method"}}},
	}

	for i, tc := range tests {
		req := httptest.NewRequest("", setQueryParams(tc.params), http.NoBody)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		output, err := Migrate(ctx)

		assert.Equal(t, tc.expOut, output, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.IsType(t, tc.expErr, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestHandler_Chdir(t *testing.T) {
	var h Handler

	dir := t.TempDir()

	tests := []struct {
		desc   string
		name   string
		expErr error
	}{
		{"valid name", dir, nil},
		{"invalid name", "", &fs.PathError{}},
	}

	for i, tc := range tests {
		err := h.Chdir(tc.name)

		assert.IsType(t, tc.expErr, err, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}

func TestHandler_OpenFile(t *testing.T) {
	var h Handler

	res, err := h.OpenFile("", os.O_RDONLY, os.FileMode(0666))

	assert.IsType(t, &os.File{}, res, "Test failed.")

	assert.IsType(t, &fs.PathError{}, err, "Test failed.")
}

func TestHandler_Stat(t *testing.T) {
	var h Handler

	dir := t.TempDir()

	tests := []struct {
		desc   string
		name   string
		expErr error
	}{
		{"valid case", dir, nil},
		{"invalid name", "", &fs.PathError{}},
	}

	for i, tc := range tests {
		_, err := h.Stat(tc.name)

		assert.IsType(t, tc.expErr, err, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}

func TestHandler_IsNotExist(t *testing.T) {
	var h Handler

	tests := []struct {
		desc    string
		err     error
		isExist bool
	}{
		{"file exits", fs.ErrNotExist, true},
		{"file not exists", &fs.PathError{}, false},
	}
	for i, tc := range tests {
		res := h.IsNotExist(tc.err)

		assert.Equal(t, tc.isExist, res, "Test case [%d] failed.", i)
	}
}

func TestHandler_GetwdError(t *testing.T) {
	var h Handler

	_, err := h.Getwd()

	assert.IsType(t, &os.SyscallError{}, err, "Test case failed")
}
