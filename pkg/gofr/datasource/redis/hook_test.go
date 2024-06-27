package redis

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryLog_PrettyPrint(t *testing.T) {
	testCases := []struct {
		desc   string
		ql     *QueryLog
		expOut []string
	}{
		{
			desc: "pipeline",
			ql: &QueryLog{
				Query:    "pipeline",
				Duration: 112,
				Args:     []interface{}{"[", "set a", "get a", "ex 300: OK", "]"},
			},
			expOut: []string{"pipeline", "112", "REDIS", "set a", "get a"},
		},
		{
			desc: "single command",
			ql: &QueryLog{
				Query:    "get",
				Duration: 22,
				Args:     []interface{}{"get", "key1"},
			},
			expOut: []string{"get", "REDIS", "22", "get key1"},
		},
	}

	for _, tc := range testCases {
		b := new(bytes.Buffer)
		tc.ql.PrettyPrint(b)

		out := b.String()

		for _, v := range tc.expOut {
			assert.Contains(t, out, v)
		}
	}
}
