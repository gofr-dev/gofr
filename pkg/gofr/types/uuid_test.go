package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func Test_ValidateUUID(t *testing.T) {
	testcases := []struct {
		uuid          []string
		expectedError error
	}{
		{[]string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}, nil},
		{[]string{"11111111-1111-1111-111111-111111111111", "22222222-2222-2222-22-222222222222"},
			errors.InvalidParam{Param: []string{"11111111-1111-1111-111111-111111111111", "22222222-2222-2222-22-222222222222"}}},
		{[]string{"", ""}, errors.InvalidParam{Param: []string{"", ""}}}}

	for i := range testcases {
		err := ValidateUUID(testcases[i].uuid[0], testcases[i].uuid[1])
		if !assert.Equal(t, err, testcases[i].expectedError) {
			t.Errorf("Testcase %v Failed. Expected: %v, Got: %v", i, testcases[i].expectedError, err)
		}
	}
}
