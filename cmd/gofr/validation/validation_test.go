package validation

import (
	"testing"

	"github.com/bmizerany/assert"

	"gofr.dev/pkg/errors"
)

func TestValidateParams(t *testing.T) {
	validParams := map[string]bool{
		"h":       true,
		"methods": true,
		"path":    true,
	}

	mandatoryParams := []string{"methods", "path"}

	testCases := []struct {
		desc   string
		params map[string]string
		expErr error
	}{
		{"success case: valid params", map[string]string{"h": "true", "methods": "GET", "path": "testPath"}, nil},
		{"failure case: invalid params", map[string]string{"invalid1": "value1"},
			&errors.Response{Reason: "unknown parameter(s) [invalid1]. Run gofr <command_name> -h for help of the command."}},
		{"failure case: missing Params", map[string]string{"methods": "GET"}, errors.MissingParam{Param: []string{"path"}}},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateParams(tc.params, validParams, &mandatoryParams)

			assert.Equalf(t, tc.expErr, err, "Test[%d] Failed.", i)
		})
	}
}
