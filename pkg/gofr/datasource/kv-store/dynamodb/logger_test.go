package dynamodb

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LogPrettyPrint(t *testing.T) {
	tests := []struct {
		desc     string
		log      Log
		expected []string
	}{
		{
			desc:     "log with key and value",
			log:      Log{Type: "GET", Duration: 120, Key: "user:1", Value: "table"},
			expected: []string{"GET", "DYNMO", "120", "user:1 table"},
		},
		{
			desc:     "log without value",
			log:      Log{Type: "DELETE", Duration: 45, Key: "user:2"},
			expected: []string{"DELETE", "DYNMO", "45", "user:2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer

			tc.log.PrettyPrint(&buf)

			for _, exp := range tc.expected {
				assert.Contains(t, buf.String(), exp)
			}
		})
	}
}
