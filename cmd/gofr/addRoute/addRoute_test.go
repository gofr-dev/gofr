package addroute

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/cmd/gofr/migration"
	gofrError "gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func Test_addRoute(t *testing.T) {
	projectPath, err := os.MkdirTemp("", "testEntity")
	if err != nil {
		t.Errorf("Received unexpected error:\n%+v", err)

		return
	}

	defer os.RemoveAll(projectPath)

	_ = os.Chdir(projectPath)
	_, _ = os.Create("main.go")

	var h Handler

	type args struct {
		methods string
		path    string
	}

	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{"success : no methods", args{"", "/hello-world"}, nil},
		{"success : one method", args{"GET", "/hello"}, nil},
		{"success : all methods", args{"ALL", "/hello-all"}, nil},
		{"success : lowercase methods", args{"get", "/hello-lower"}, nil},
		{"success : multiple methods", args{"GET,POST", "/test"}, nil},
	}

	for _, tt := range tests {
		_ = os.Chdir(projectPath)

		err := addRoute(h, tt.args.methods, tt.args.path)
		if err != nil && (err.Error() != tt.wantErr.Error()) {
			t.Errorf("Test %v: addRoute() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()

	_ = os.Chdir(dir)
	_, _ = os.Create("main.go")

	var h Handler

	type args struct {
		path   string
		method string
	}

	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{"invalid path", args{"/$/{id}", http.MethodGet}, invalidPathError{"$/{id}"}},
		{"invalid method", args{"/abcd/{id}", http.MethodPatch}, invalidMethodError{"PATCH"}},
	}

	for _, tt := range tests {
		_ = os.Chdir(dir)

		err := addRoute(h, tt.args.method, tt.args.path)
		if err != nil && err.Error() != tt.wantErr.Error() {
			t.Errorf("Test %v: addRoute() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestErrors_FileSystem(t *testing.T) {
	dir := t.TempDir()

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)
	_ = os.Chdir(dir)
	test, _ := os.Create("test.go")

	type args struct {
		path   string
		method string
	}

	tests := []struct {
		name      string
		args      args
		mockCalls []*gomock.Call
		wantErr   bool
	}{
		{"error: Match error", args{"/brand", http.MethodGet}, []*gomock.Call{
			c.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false, errors.New("test error")).Times(1),
		}, true},

		{"error: OpenFile", args{"/brand", http.MethodGet}, []*gomock.Call{
			c.EXPECT().Match(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1),
		}, true},

		{"error: Getwd", args{"/brand", http.MethodGet}, []*gomock.Call{
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(test, nil).AnyTimes(),
			c.EXPECT().Getwd().Return("", errors.New("test error")).Times(1),
		}, true},
	}

	for _, tt := range tests {
		if err := addRoute(c, tt.args.method, tt.args.path); (err != nil) != tt.wantErr {
			t.Errorf("Test %v: addRoute() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func Test_addHandlerImport(t *testing.T) {
	dir := t.TempDir()

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)
	_ = os.Chdir(dir)

	testFile, _ := os.Create("testing.go")

	data := []byte("gofr.dev/pkg/gofr")

	_, _ = testFile.Write(data)

	file, _ := os.OpenFile("testing.go", os.O_CREATE|os.O_RDWR, migration.RWMode)

	type args struct {
		mainString string
	}

	tests := []struct {
		name      string
		args      args
		mockCalls []*gomock.Call
		wantErr   bool
	}{
		{"Success OpenFile", args{"package main"}, []*gomock.Call{
			c.EXPECT().Getwd().Return(dir+"/testEntity", nil).MaxTimes(1),
			c.EXPECT().Chdir(gomock.Any()).Return(nil).MaxTimes(1),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(file, nil).MaxTimes(1),
		}, false},
		{"Error OpenFile", args{"package main"}, []*gomock.Call{
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1),
		}, true},
	}

	for i, tt := range tests {
		if err := addHandlerImport(c, tt.args.mainString, ""); (err != nil) != tt.wantErr {
			t.Errorf("Test[%d] %v: populateMain() error = %v, wantErr %v", i, tt.name, err, tt.wantErr)
		}
	}
}

func Test_populateMain(t *testing.T) {
	dir := t.TempDir()

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)
	_ = os.Chdir(dir)

	testFile, _ := os.Create("testing.go")
	testFile1, _ := os.OpenFile("testing1.go", os.O_CREATE|os.O_RDONLY, migration.RWMode)

	data := []byte("Hello, World!\ngofr.Start()")

	_, _ = testFile.Write(data)

	file, _ := os.OpenFile("testing.go", os.O_CREATE|os.O_RDWR, migration.RWMode)

	type args struct {
		mainString string
	}

	tests := []struct {
		name      string
		args      args
		mockCalls []*gomock.Call
		wantErr   bool
	}{
		{"Error chdir", args{"package main"}, []*gomock.Call{
			c.EXPECT().Getwd().Return(dir+"/testEntity", nil).MaxTimes(5),
			c.EXPECT().Chdir(gomock.AssignableToTypeOf(dir)).Return(errors.New("test error")),
		}, true},

		{"Error OpenFile", args{"package main"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.AssignableToTypeOf(dir)).Return(nil).MaxTimes(1),
			c.EXPECT().OpenFile("main.go", gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")),
		}, true},

		{"Error OpenFile", args{"package main"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.AssignableToTypeOf(dir)).Return(nil).MaxTimes(1),
			c.EXPECT().OpenFile("main.go", gomock.Any(), gomock.Any()).Return(nil, nil),
		}, true},

		{"Error OpenFile", args{"package main"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.AssignableToTypeOf(dir)).Return(nil).MaxTimes(1),
			c.EXPECT().OpenFile("main.go", gomock.Any(), gomock.Any()).Return(testFile1, nil),
		}, true},

		{"Success OpenFile", args{"package main"}, []*gomock.Call{
			c.EXPECT().Getwd().Return(dir+"/testEntity", nil).MaxTimes(1),
			c.EXPECT().Chdir(gomock.AssignableToTypeOf(dir)).Return(nil).MaxTimes(1),
			c.EXPECT().OpenFile("main.go", gomock.Any(), gomock.Any()).Return(file, nil).Times(2),
		}, false},
	}

	for i, tt := range tests {
		if err := populateMain(c, tt.args.mainString, ""); (err != nil) != tt.wantErr {
			t.Errorf("Test[%d] %v: populateMain() error = %v, wantErr %v", i, tt.name, err, tt.wantErr)
		}
	}
}

func Test_populateHandler(t *testing.T) {
	dir := t.TempDir()
	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)

	_ = os.Chdir(dir)

	testFile, _ := os.OpenFile("testing.go", os.O_CREATE|os.O_RDONLY, migration.RWMode)

	type args struct {
		path          string
		handlerString string
	}

	arg := args{"brand", "package brand"}

	tests := []struct {
		desc     string
		mockCall []*gomock.Call
	}{
		{"chdir error:http directory", []*gomock.Call{
			c.EXPECT().Stat(gomock.Any()).Return(nil, nil).MaxTimes(9),
			c.EXPECT().IsNotExist(gomock.Any()).Return(false).MaxTimes(9),
			c.EXPECT().Chdir("http").Return(errors.New("test error")),
		}},

		{"chdir error: path directory", []*gomock.Call{
			c.EXPECT().Chdir("http").Return(nil),
			c.EXPECT().Chdir("brand").Return(errors.New("test error")),
		}},

		{"openfile error", []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).MaxTimes(2),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")),
		}},

		{"openfile error: nil returned", []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).MaxTimes(2),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
		}},

		{"openfile error: read only file given to write the content", []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).MaxTimes(2),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(testFile, nil),
		}},
	}

	for i, tc := range tests {
		if err := populateHandler(c, arg.path, arg.handlerString); err == nil {
			t.Errorf("TEST [%d] failed. populateHandler(), Got: %v, Expected: %v", i, nil, tc.desc)
		}
	}
}

func Test_createChangeDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c := NewMockfileSystem(ctrl)
	c.EXPECT().Stat(gomock.Any()).Return(nil, errors.New("test error"))
	c.EXPECT().IsNotExist(gomock.Any()).Return(true)
	c.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(errors.New("test error"))

	if err := createChangeDir(c, "test"); err == nil {
		t.Errorf("Test failed : createChangeDir(), Got : %v, Expected: %v", nil, "error in changing directory.")
	}
}

func Test_existCheck(t *testing.T) {
	type args struct {
		file io.ReadSeeker
		elem string
	}

	arg := args{nil, ""}

	count, isExist := existCheck(arg.file, arg.elem)

	assert.Equal(t, 0, count, "Test failed")

	assert.Equal(t, false, isExist, "Test failed")
}

func Test_importSortCheck(t *testing.T) {
	tests := []struct {
		desc       string
		directory  string
		lineString string
		expOut     string
	}{
		{"lineString+directory sort check", "gofr.dev/pkg/gofr", "gofr.dev/pkg/gofr",
			"gofr.dev/pkg/gofr\n\t\"gofr.dev/pkg/gofr\""},
		{"directory+lineString sort check", "abc", "gofr.dev/pkg/gofr",
			"\t\"abc\"\ngofr.dev/pkg/gofr"},
	}

	for i, tc := range tests {
		output := importSortCheck(tc.directory, tc.lineString)

		assert.Equal(t, tc.expOut, output, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		desc   string
		err    error
		expMsg string
	}{
		{"path already exist", pathExistsError{path: "/api", method: "Get", line: 1, file: "gofr"},
			"route /api is already present for the methods:-  Get at line number: 1 in file: gofr"},
		{"invalid method", invalidMethodError{"updates"}, "updates is not a valid method"},
		{"invalid path", invalidPathError{"/gofr.dev/gofr"}, "/gofr.dev/gofr is an invalid path"},
	}

	for i, tc := range tests {
		msg := tc.err.Error()

		assert.Equal(t, tc.expMsg, msg, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}

func Test_AddRoute(t *testing.T) {
	dir := t.TempDir()

	_ = os.Chdir(dir)
	_, _ = os.Create("main.go")

	var h Handler
	help := h.Help()

	tests := []struct {
		desc      string
		params    map[string]string
		expOutput interface{}
		err       interface{}
	}{
		{"Success Case: add route", map[string]string{"methods": "POST", "path": "/test"}, "Added route: /test", nil},
		{"Success Case: helper case", map[string]string{"h": "true", "methods": "GET", "path": "/help"}, help, nil},
		{"Success Case: all methods", map[string]string{"h": "true", "methods": "ALL", "path": "/all"}, help, nil},
		{"Success Case: lowercase methods", map[string]string{"h": "true", "methods": "get", "path": "/lower"}, help, nil},
		{"Failure Case: invalid method", map[string]string{"methods": "PATCH", "path": "/help"}, nil, invalidMethodError{"PATCH"}},
		{"Failure Case: missing path", map[string]string{"methods": "GET"}, nil, gofrError.MissingParam{Param: []string{"path"}}},
		{"Failure Case: missing methods", map[string]string{"path": "/test"}, nil, gofrError.MissingParam{Param: []string{"methods"}}},
		{"Failure Case: invalid param", map[string]string{"invalid": "value"}, nil,
			&gofrError.Response{Reason: "unknown parameter(s) [invalid]. Run gofr <command_name> -h for help of the command."}},
	}

	for i, tc := range tests {
		_ = os.Chdir(dir)

		req := httptest.NewRequest("", setQueryParams(tc.params), nil)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		res, err := AddRoute(ctx)

		assert.Equalf(t, tc.expOutput, res, "TEST[%d] failed.\n%s", i, tc.desc)

		assert.Equalf(t, tc.err, err, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}

func setQueryParams(params map[string]string) string {
	url := "/dummy?"

	for key, value := range params {
		url = fmt.Sprintf("%s%s=%s&", url, key, value)
	}

	return strings.TrimSuffix(url, "&")
}
