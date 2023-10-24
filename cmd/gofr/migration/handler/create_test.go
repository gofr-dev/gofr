package handler

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/cmd/gofr/migration"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func Test_CreateMigration(t *testing.T) {
	currDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())

	// revert current directory
	defer func() { _ = os.Chdir(currDir) }()

	h := Create{}
	help := h.Help()

	testCases := []struct {
		desc   string
		params map[string]string
		expRes interface{}
	}{
		{"success case: help", map[string]string{"h": "true"}, help},
		{"success case: migration created", map[string]string{"name": "testMigration"}, "Migration created: testMigration"},
	}

	for i, tc := range testCases {
		req := httptest.NewRequest("", setQueryParams(tc.params), nil)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		res, err := CreateMigration(ctx)

		assert.Nilf(t, err, "Test[%d] Failed", i)
		assert.Equalf(t, tc.expRes, res, "Test[%d] Failed", i)
	}
}
func Test_create_success(t *testing.T) {
	rwMode := os.FileMode(migration.RWMode)

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	dir := t.TempDir()

	allFiles, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("error %s was not expected while opening allFiles", err)
	}

	mockFS := NewMockFSCreate(ctrl)

	file1, _ := os.OpenFile(path.Join(dir, "test1.txt"), os.O_CREATE|os.O_WRONLY, migration.RWMode)
	file2, _ := os.OpenFile(path.Join(dir, "test2.txt"), os.O_CREATE|os.O_WRONLY, migration.RWMode)

	mockFS.EXPECT().Stat("migrations").Return(nil, &errors.Response{Reason: "test error"})
	mockFS.EXPECT().IsNotExist(&errors.Response{Reason: "test error"}).Return(false)
	mockFS.EXPECT().Chdir("migrations").Return(nil)
	mockFS.EXPECT().OpenFile(gomock.Any(), os.O_CREATE|os.O_WRONLY, rwMode).Return(file1, nil)
	mockFS.EXPECT().ReadDir("./").Return(allFiles, nil)
	mockFS.EXPECT().Create("000_all.go").Return(file2, nil)

	err = create(mockFS, "testing")

	assert.Nil(t, err, "Test case failed. got: %v, expected: %v", err, "nil")
}

func Test_create_fail(t *testing.T) {
	var (
		rwxMode = os.FileMode(migration.RWXMode)
		rwMode  = os.FileMode(migration.RWMode)
		expErr  = &errors.Response{Reason: "test error"}
	)

	ctrl := gomock.NewController(t)

	defer func() {
		ctrl.Finish()
	}()

	dir := t.TempDir()

	allFiles, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("error %s was not expected while opening allFiles", err)
	}

	file, _ := os.OpenFile(path.Join(dir, "test.txt"), os.O_CREATE|os.O_WRONLY, migration.RWMode)
	file1, _ := os.OpenFile(path.Join(dir, "test1.txt"), os.O_CREATE|os.O_WRONLY, migration.RWMode)

	mockFS := NewMockFSCreate(ctrl)

	tests := []struct {
		desc      string
		fileName  string
		mockCalls []*gomock.Call
	}{
		{"mkdir error", "testing", []*gomock.Call{
			mockFS.EXPECT().Stat("migrations").Return(nil, &errors.Response{Reason: "test error"}).MaxTimes(6),
			mockFS.EXPECT().IsNotExist(&errors.Response{Reason: "test error"}).Return(true).MaxTimes(6),
			mockFS.EXPECT().Mkdir("migrations", rwxMode).Return(expErr),
		}},
		{"chdir error", "testing", []*gomock.Call{
			mockFS.EXPECT().Mkdir("migrations", rwxMode).Return(nil).MaxTimes(5),
			mockFS.EXPECT().Chdir("migrations").Return(expErr),
		}},
		{"unable to openfile", "testing", []*gomock.Call{
			mockFS.EXPECT().Chdir("migrations").Return(nil).MaxTimes(4),
			mockFS.EXPECT().OpenFile(gomock.Any(), os.O_CREATE|os.O_WRONLY, rwMode).Return(nil, expErr),
		}},
		{"error OpenFile: nil returned", "testing", []*gomock.Call{
			mockFS.EXPECT().OpenFile(gomock.Any(), os.O_CREATE|os.O_WRONLY, rwMode).Return(nil, nil),
		}},
		{"read dir error", "testing", []*gomock.Call{
			mockFS.EXPECT().OpenFile(gomock.Any(), os.O_CREATE|os.O_WRONLY, rwMode).Return(file, nil),
			mockFS.EXPECT().ReadDir(gomock.Any()).Return(nil, expErr),
		}},
		{"create file error", "testing", []*gomock.Call{
			mockFS.EXPECT().OpenFile(gomock.Any(), os.O_CREATE|os.O_WRONLY, rwMode).Return(file1, nil),
			mockFS.EXPECT().ReadDir("./").Return(allFiles, nil),
			mockFS.EXPECT().Create("000_all.go").Return(nil, expErr),
		}},
	}

	for i, tc := range tests {
		if err := create(mockFS, tc.fileName); err == nil {
			t.Errorf("Test case [%d] failed. got: %v, expected: %v", i, nil, tc.desc)
		}
	}
}

func Test_getPrefixes(t *testing.T) {
	dir := t.TempDir()

	// these files will be ignored
	_, _ = os.Create(path.Join(dir, "20190320095356_test.go"))                       // files with len < 2 will be ignored
	_, _ = os.Create(path.Join(dir, "000_all.go"))                                   // 000_all.go will be ignored.
	_, _ = os.Create(path.Join(dir, "20220320095352_table_employee_create_test.go")) // ignores the files that have the suffix test

	// files whose prefixes will be added to the slice prefixes
	_, _ = os.Create(path.Join(dir, "20220410095352_table_employee_create.go"))
	_, _ = os.Create(path.Join(dir, "20210520095352_table_employee_create.go"))
	_, _ = os.Create(path.Join(dir, "20190320095352_table_employee_create.go"))

	allFiles, _ := os.ReadDir(dir)
	ctrl := gomock.NewController(t)
	mockFS := NewMockFSCreate(ctrl)

	tests := []struct {
		desc      string
		output    []string
		err       error
		mockEntry []os.DirEntry
	}{
		{"Success", []string{"20190320095352", "20210520095352", "20220410095352"}, nil, allFiles},
		{"Error in Reading files", nil, errors.Error("Error while reading file"), nil},
	}

	for i, tc := range tests {
		mockFS.EXPECT().ReadDir("./").Return(tc.mockEntry, tc.err)

		result, err := getPrefixes(mockFS)

		assert.Equal(t, tc.output, result, "Test case [%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.err, err, "Test case [%d], failed.\n%s", i, tc.desc)
	}
}

func Test_CreateMigrationValidationFail(t *testing.T) {
	testCases := []struct {
		desc   string
		params map[string]string
		expErr error
	}{
		{"failure case: invalid params", map[string]string{"invalid": "value", "name": "testName"},
			&errors.Response{Reason: "unknown parameter(s) [invalid]. Run gofr <command_name> -h for help of the command."}},
		{"failure case: missing Params", map[string]string{}, errors.MissingParam{Param: []string{"name"}}},
	}

	for i, tc := range testCases {
		req := httptest.NewRequest("", setQueryParams(tc.params), nil)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), nil)

		res, err := CreateMigration(ctx)

		assert.Nil(t, res)
		assert.EqualErrorf(t, tc.expErr, err.Error(), "Test[%d] Failed", i)
	}
}

func setQueryParams(params map[string]string) string {
	url := "/dummy?"

	for key, value := range params {
		url = fmt.Sprintf("%s%s=%s&", url, key, value)
	}

	return strings.TrimSuffix(url, "&")
}
