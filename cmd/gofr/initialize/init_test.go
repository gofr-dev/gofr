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

	fileSys := NewMockfileSystem(ctrl)

	f, _ := os.Create("main.go")
	defer os.Remove("main.go")

	type args struct {
		f    fileSystem
		name string
	}

	arg := args{f: fileSys, name: "testProject"}

	tests := []struct {
		desc      string
		mockCalls []*gomock.Call
		expErr    error
	}{
		{
			desc: "Error Mkdir",
			mockCalls: []*gomock.Call{
				fileSys.EXPECT().Mkdir("testProject", gomock.Any()).Return(errors.New("test error")),
			},
			expErr: errors.New("test error"),
		},
		{
			desc: "Error Chdir",
			mockCalls: []*gomock.Call{
				fileSys.EXPECT().Mkdir("testProject", gomock.Any()).Return(nil),
				fileSys.EXPECT().Chdir(gomock.Any()).Return(errors.New("test error")),
			},
			expErr: errors.New("test error"),
		},
		{
			desc: "Error Mkdir - Standard Directories",
			mockCalls: []*gomock.Call{
				fileSys.EXPECT().Mkdir("testProject", gomock.Any()).Return(nil),
				fileSys.EXPECT().Chdir("testProject").Return(nil),
				fileSys.EXPECT().Mkdir("cmd", gomock.Any()).Return(nil),
				fileSys.EXPECT().Mkdir("configs", gomock.Any()).Return(errors.New("test error")),
			},
			expErr: errors.New("test error"),
		},
		{
			desc: "Error Create",
			mockCalls: []*gomock.Call{
				fileSys.EXPECT().Mkdir("testProject", gomock.Any()).Return(nil),
				fileSys.EXPECT().Chdir("testProject").Return(nil),
				fileSys.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(nil).Times(3),
				fileSys.EXPECT().Create(gomock.Any()).Return(nil, errors.New("create error")),
			},
			expErr: errors.New("create error"),
		},
		{
			desc: "Error WriteString",
			mockCalls: []*gomock.Call{
				fileSys.EXPECT().Mkdir("testProject", gomock.Any()).Return(nil),
				fileSys.EXPECT().Chdir("testProject").Return(nil),
				fileSys.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(nil).Times(3),
				fileSys.EXPECT().Create(gomock.Any()).Return(nil, nil),
			},
			expErr: errors.New("invalid argument"),
		},
		{
			desc: "Error create env files",
			mockCalls: []*gomock.Call{
				fileSys.EXPECT().Mkdir("testProject", gomock.Any()).Return(nil),
				fileSys.EXPECT().Chdir("testProject").Return(nil),
				fileSys.EXPECT().Mkdir(gomock.Any(), gomock.Any()).Return(nil).Times(3),
				fileSys.EXPECT().Create(gomock.Any()).Return(f, nil),
				fileSys.EXPECT().Chdir("configs").Return(errors.New("test error")),
			},
			expErr: errors.New("test error"),
		},
	}

	for i, tc := range tests {
		err := createProject(arg.f, arg.name)
		if err != nil && err.Error() != tc.expErr.Error() {
			t.Errorf("TEST[%d] failed. createProject(), Got: %v, Expected: %v", i, err, tc.expErr)
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

func Test_createEnvFiles_Errors(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	fileSys := NewMockfileSystem(ctrl)

	testCases := []struct {
		desc     string
		mockCall []*gomock.Call
		expErr   error
	}{
		{
			desc: "Failed switching to configs dir",
			mockCall: []*gomock.Call{
				fileSys.EXPECT().Chdir("configs").Return(errors.New("test error")),
			},
			expErr: errors.New("test error"),
		},
		{
			desc: "Failed creating env file",
			mockCall: []*gomock.Call{
				fileSys.EXPECT().Chdir("configs").Return(nil),
				fileSys.EXPECT().Create(".env").Return(nil, errors.New("create error")),
			},
			expErr: errors.New("create error"),
		},
	}

	for i, tc := range testCases {
		err := createEnvFiles(fileSys, []string{".env"})

		assert.Equalf(t, tc.expErr, err, "Test casee [%d] failed", i)
	}
}

func setQueryParams(params map[string]string) string {
	url := "/dummy?"

	for key, value := range params {
		url = fmt.Sprintf("%s%s=%s&", url, key, value)
	}

	return strings.TrimSuffix(url, "&")
}
