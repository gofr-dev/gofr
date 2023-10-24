package initialize

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

func Test_createProjectErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fileSys := NewMockFileSystem(ctrl)

	type args struct {
		f    fileSystem
		name string
	}

	arg := args{fileSys, "testProject"}

	tests := []struct {
		desc      string
		mockCalls []*gomock.Call
	}{
		{"Error Mkdir", []*gomock.Call{
			fileSys.EXPECT().Mkdir("testProject", gomock.Any()).Return(errors.New("test error")),
		}},

		{"Error Chdir", []*gomock.Call{
			fileSys.EXPECT().Mkdir("testProject", gomock.Any()).Return(nil),
			fileSys.EXPECT().Chdir(gomock.Any()).Return(errors.New("test error")),
		}},

		{"Error Mkdir - Standard Directories", []*gomock.Call{
			fileSys.EXPECT().Chdir("testProject").Return(nil).MaxTimes(3),
			fileSys.EXPECT().Mkdir("configs", gomock.Any()).Return(errors.New("test error")),
			fileSys.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(10),
		}},

		{"Error Create", []*gomock.Call{
			fileSys.EXPECT().Create(gomock.Any()).Return(nil, errors.New("test error")),
		}},

		{"Error WriteString", []*gomock.Call{
			fileSys.EXPECT().Create(gomock.Any()).Return(nil, nil),
		}},
	}

	for i, tc := range tests {
		if err := createProject(arg.f, arg.name); err == nil {
			t.Errorf("TEST[%d] failed. createProject(), Got: %v, Expected: %v", i, nil, tc.desc)
		}
	}
}

func Test_createProject(t *testing.T) {
	var h Handler

	type args struct {
		f    fileSystem
		name string
	}

	arg := args{h, "testProject"}

	tests := []struct {
		desc   string
		expErr bool
	}{
		{"Success Case", false},
		{"Project with same name already exists error", true},
	}

	dir := t.TempDir()

	for i, tc := range tests {
		_ = os.Chdir(dir)

		if err := createProject(h, arg.name); (err != nil) != tc.expErr {
			t.Errorf("TEST[%d] failed. createProject(), Got: %v, Expected: %v", i, err, tc.expErr)
		}
	}
}

func Test_Init(t *testing.T) {
	path, err := os.MkdirTemp("", "testInit")
	if err != nil {
		t.Errorf("Received unexpected error:%v", err)
	}

	defer os.RemoveAll(path)

	var h Handler
	help := h.Help()

	tests := []struct {
		desc        string
		params      map[string]string
		expOut      interface{}
		expectedErr error
	}{
		{"Success Case: create project", map[string]string{"name": "framework"}, "Successfully created project: framework", nil},
		{"Success Case: help case", map[string]string{"h": "true", "name": "bulkFramework"}, help, nil},
		{"Failure Case: missing param", map[string]string{}, nil, gofrError.MissingParam{Param: []string{"name"}}},
		{"Failure Case: invalid param", map[string]string{"invalid": "value"}, nil,
			&gofrError.Response{Reason: "unknown parameter(s) [invalid]. Run gofr <command_name> -h for help of the command."}},
	}

	for i, tc := range tests {
		_ = os.Chdir(path)

		req := httptest.NewRequest("", setQueryParams(tc.params), nil)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		res, err := Init(ctx)

		assert.Equalf(t, tc.expOut, res, "Test case [%d] failed.", i)
		assert.IsTypef(t, tc.expectedErr, err, "Test case [%d] failed.", i)
	}
}

func setQueryParams(params map[string]string) string {
	url := "/dummy?"

	for key, value := range params {
		url = fmt.Sprintf("%s%s=%s&", url, key, value)
	}

	return strings.TrimSuffix(url, "&")
}
