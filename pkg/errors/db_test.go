package errors

import (
	"errors"
	"testing"
)

func TestDb_Error(t *testing.T) {
	testcases := []struct {
		err error
		exp string
	}{
		{DB{}, "DB Error"},
		{DB{Err: errors.New("sql error")}, "sql error"},
	}

	for i := range testcases {
		err := testcases[i].err.Error()
		if err != testcases[i].exp {
			t.Errorf("[TESTCASE %v]Failed. Expected %v\nGot %v", i+1, testcases[i].exp, err)
		}
	}
}
