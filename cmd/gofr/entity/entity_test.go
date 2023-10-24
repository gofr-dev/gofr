package entity

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	gofrError "gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func Test_addEntity(t *testing.T) {
	path, err := os.MkdirTemp("", "testEntity")
	if err != nil {
		t.Errorf("Received unexpected error:\n%+v", err)

		return
	}

	defer os.RemoveAll(path)

	_ = os.Chdir(path)

	var h Handler

	type args struct {
		entity     string
		entityType string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"Invalid value for flag type ", args{"brand", "store"}, true},
		{"Success Case: core", args{"brand", "core"}, false},
		{"Success Case: composite", args{"brand", "composite"}, false},
		{"Success Case: consumer", args{"brand", "consumer"}, false},
	}

	for _, tt := range tests {
		_ = os.Chdir(path)

		if err := addEntity(h, tt.args.entityType, tt.args.entity); (err != nil) != tt.wantErr {
			t.Errorf("Test %v: addEntity() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}

	{
		err := addEntity(h, "store", "brand")
		if err != nil && (err.Error() != invalidTypeError{}.Error()) {
			t.Errorf("invalid type error expected")
		}
	}
}

func TestErrors_addCore(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)

	path, err := os.MkdirTemp("", "testEntity")
	if err != nil {
		t.Errorf("Received unexpected error:\n%+v", err)

		return
	}

	defer os.RemoveAll(path)

	test, _ := os.Create(path + "/test.txt")
	testingFile, _ := os.Create(path + "/testingFile.txt")

	type args struct {
		name       string
		entityType string
	}

	tests := []struct {
		name        string
		args        args
		mockedCalls []*gomock.Call
		wantErr     bool
	}{
		{"error : Getwd()", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().Getwd().Return("", errors.New("test error")).Times(1),
		}, true},

		{"error : createChangeDir()", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().Getwd().Return(path, nil).AnyTimes(),
			c.EXPECT().Stat(gomock.Any()).Return(nil, errors.New("doesn't exist")).Times(1),
			c.EXPECT().IsNotExist(gomock.Any()).Return(true).Times(1),
			c.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(errors.New("test error")).Times(1),
		}, true},

		{"error composite: createChangeDir()", args{"brand", "composite"}, []*gomock.Call{
			c.EXPECT().Stat(gomock.Any()).Return(nil, errors.New("doesn't exist")).Times(1),
			c.EXPECT().IsNotExist(gomock.Any()).Return(true).Times(1),
			c.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(errors.New("test error")).Times(1),
		}, true},

		{"error consumer: createChangeDir()", args{"brand", "consumer"}, []*gomock.Call{
			c.EXPECT().Stat(gomock.Any()).Return(nil, errors.New("doesn't exist")).Times(1),
			c.EXPECT().IsNotExist(gomock.Any()).Return(true).Times(1),
			c.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(errors.New("test error")).Times(1),
		}, true},

		{"error: Chdir", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().IsNotExist(gomock.Any()).Return(false).AnyTimes(),
			c.EXPECT().Stat(gomock.Any()).Return(nil, nil).AnyTimes(),
			c.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().Chdir(path + "/core/brand").Return(errors.New("test error")).Times(1),
			c.EXPECT().Chdir(path + "/core").Return(nil).Times(1),
			c.EXPECT().OpenFile("interface.go", gomock.Any(), gomock.Any()).Return(test, nil).Times(1),
		}, true},

		{"error: OpenFile", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).AnyTimes(),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1),
		}, true},

		{"error: OpenFile, filePtr is nil", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
		}, true},

		{"error: OpenFile, entity file open error", args{"brand", "core"}, []*gomock.Call{
			c.EXPECT().OpenFile("brand.go", gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(testingFile, nil).AnyTimes(),
		}, true},
	}

	for _, tt := range tests {
		_ = os.Chdir(path)

		if err := addEntity(c, tt.args.entityType, tt.args.name); (err != nil) != tt.wantErr {
			t.Errorf("Test %v: addEntity() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func Test_addConsumer(t *testing.T) {
	projectDirectory, err := os.MkdirTemp("", "testProject")
	if err != nil {
		t.Errorf("Received unexpected error:%v", err)

		return
	}

	defer os.RemoveAll(projectDirectory)

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)

	type args struct {
		entity string
	}

	arg := args{"product"}

	tests := []struct {
		desc     string
		mockCall []*gomock.Call
	}{
		{"error http changeDir", []*gomock.Call{
			c.EXPECT().Stat(gomock.Any()).Return(nil, nil).MaxTimes(7),
			c.EXPECT().IsNotExist(gomock.Any()).Return(false).MaxTimes(7),
			c.EXPECT().Chdir(projectDirectory + "/http").Return(errors.New("test error")),
		}},

		{"error entity changeDir", []*gomock.Call{
			c.EXPECT().Chdir("product").Return(errors.New("test error")),
			c.EXPECT().Chdir(gomock.Any()).Return(nil).MaxTimes(5),
		}},

		{"error OpenFile", []*gomock.Call{
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")),
		}},

		{"error OpenFile: nil returned", []*gomock.Call{
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
		}},
	}

	for i, tc := range tests {
		if err := addConsumer(c, projectDirectory, arg.entity); err == nil {
			t.Errorf("TEST[%d] failed. addConsumer(), Got: %v, Expected: %v", i, nil, tc.desc)
		}
	}
}

func Test_addComposite(t *testing.T) {
	path, err := os.MkdirTemp("", "testProject")
	if err != nil {
		t.Errorf("Received unexpected error:%v", err)

		return
	}

	defer os.RemoveAll(path)

	_ = os.Chdir(path)

	testFile, err := os.CreateTemp(path, "test.go")
	if err != nil {
		t.Errorf("Received unexpected error:%v", err)

		return
	}

	compositePath := path + "/composite"

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	c := NewMockfileSystem(ctrl)

	tests := []struct {
		desc     string
		mockCall []*gomock.Call
	}{
		{"error Chdir: composite directory", []*gomock.Call{
			c.EXPECT().Stat(gomock.Any()).Return(nil, nil).MaxTimes(5),
			c.EXPECT().IsNotExist(gomock.Any()).Return(false).MaxTimes(5),
			c.EXPECT().Chdir(gomock.Any()).Return(errors.New("test error")),
		}},

		{"error OpenFile", []*gomock.Call{
			c.EXPECT().Chdir(gomock.Any()).Return(nil).MaxTimes(2),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")),
		}},

		{"error OpenFile: nil returned", []*gomock.Call{
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
		}},

		{"error Chdir: entity directory", []*gomock.Call{
			c.EXPECT().Chdir(compositePath + "/brand").Return(errors.New("test error")),
			c.EXPECT().Chdir(gomock.Any()).Return(nil),
			c.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(testFile, nil),
		}},
	}

	for i, tc := range tests {
		if err := addComposite(c, path, "brand"); err == nil {
			t.Errorf("TEST[%d] failed. addComposite(), Got: %v, Expected: %v", i, nil, tc.desc)
		}
	}
}

func Test_AddEntity(t *testing.T) {
	path, err := os.MkdirTemp("", "testEntity")
	if err != nil {
		t.Errorf("Received unexpected error:%v", err)

		return
	}

	defer os.RemoveAll(path)

	var h Handler
	help := h.Help()

	tests := []struct {
		desc      string
		params    map[string]string
		expOutput interface{}
		err       error
	}{
		{"Success Case: entity created", map[string]string{"type": "core", "name": "gofr"}, "Successfully created entity: gofr", nil},
		{"Success Case: help case", map[string]string{"h": "TRUE", "type": "core"}, help, nil},
		{"Failure Case: invalid type", map[string]string{"type": "comment"}, nil, invalidTypeError{}},
		{"Failure Case: missing param", map[string]string{}, nil, gofrError.MissingParam{Param: []string{"type"}}},
		{"Failure Case: invalid param", map[string]string{"invalid": "value"}, nil,
			&gofrError.Response{Reason: "unknown parameter(s) [invalid]. Run gofr <command_name> -h for help of the command."}},
	}

	for i, tc := range tests {
		_ = os.Chdir(path)

		req := httptest.NewRequest("", setQueryParams(tc.params), nil)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		res, err := AddEntity(ctx)

		assert.Equalf(t, tc.expOutput, res, "Test case [%d] failed.", i)

		assert.Equalf(t, tc.err, err, "Test case [%d] failed.", i)
	}
}

func setQueryParams(params map[string]string) string {
	url := "/dummy?"

	for key, value := range params {
		url = fmt.Sprintf("%s%s=%s&", url, key, value)
	}

	return strings.TrimSuffix(url, "&")
}
