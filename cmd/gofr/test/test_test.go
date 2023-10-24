package test

import (
	"fmt"
	"io/fs"
	"net/http/httptest"
	"net/url"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func Test_GenerateIntegrationTest(t *testing.T) {
	help := testHelp()

	testCases := []struct {
		desc        string
		params      map[string]string
		expRes      interface{}
		expectedErr error
	}{
		{"success case: help", map[string]string{"h": "true"}, help, nil},
		{"failure case: source error", map[string]string{"source": "invalid", "host": "127.0.0.1"}, nil,
			&fs.PathError{Op: "open", Path: "invalid", Err: syscall.ENOENT}},
		{"failure case: invalid host",
			map[string]string{"source": "openapi.json", "host": "invalid-Host"},
			"Test Failed!", &url.Error{},
		},
		{"success case: migration created",
			map[string]string{"source": "openapi.json", "host": "jsonplaceholder.typicode.com"},
			"Test Passed!", nil,
		},
	}

	for i, tc := range testCases {
		req := httptest.NewRequest("", setQueryParams(tc.params), nil)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		res, err := GenerateIntegrationTest(ctx)

		assert.IsTypef(t, tc.expectedErr, err, "Test[%d] Failed. %v", i, tc.desc)
		assert.Equalf(t, tc.expRes, res, "Test[%d] Failed. %v", i, tc.desc)
	}
}

func Test_GenerateIntegrationTestValidationFail(t *testing.T) {
	testCases := []struct {
		desc   string
		params map[string]string
		expErr error
	}{
		{"failure case: invalid params", map[string]string{"invalid": "value", "source": "test", "host": "testDB"},
			&errors.Response{Reason: "unknown parameter(s) [invalid]. Run gofr <command_name> -h for help of the command."}},
		{"failure case: missing Params", map[string]string{"source": "testDB"}, errors.MissingParam{Param: []string{"host"}}},
	}

	for i, tc := range testCases {
		req := httptest.NewRequest("", setQueryParams(tc.params), nil)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		res, err := GenerateIntegrationTest(ctx)

		assert.Nil(t, res)
		assert.EqualErrorf(t, tc.expErr, err.Error(), "Test[%d] Failed", i)
	}
}

func setQueryParams(params map[string]string) string {
	dummyURL := "/dummy?"

	for key, value := range params {
		dummyURL = fmt.Sprintf("%s%s=%s&", dummyURL, key, value)
	}

	return strings.TrimSuffix(dummyURL, "&")
}
