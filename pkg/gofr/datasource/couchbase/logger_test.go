package couchbase

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryLog_PrettyPrint(t *testing.T) {
	tests := []struct {
		name        string
		log         QueryLog
		wantEmpty   bool
		wantContain []string
	}{
		{
			name:      "no key and no statement prints nothing",
			log:       QueryLog{Query: "Get", Duration: 10},
			wantEmpty: true,
		},
		{
			name:        "key with nil parameters",
			log:         QueryLog{Query: "Get", Key: "user:1", Duration: 42},
			wantContain: []string{"Get", "COUCHBASE", "42", "user:1"},
		},
		{
			name:        "statement used when query is empty",
			log:         QueryLog{Statement: "SELECT   *\n FROM  `bucket`", Parameters: map[string]any{"id": 1}, Duration: 7},
			wantContain: []string{"SELECT * FROM `bucket`", "COUCHBASE", "map[id:1]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			tt.log.PrettyPrint(&buf)

			assert.Equal(t, tt.wantEmpty, buf.Len() == 0)

			for _, s := range tt.wantContain {
				assert.Contains(t, buf.String(), s)
			}
		})
	}
}

func Test_clean(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "collapses whitespace", input: "SELECT  *\n\tFROM   b", want: "SELECT * FROM b"},
		{name: "trims leading and trailing whitespace", input: "  key  ", want: "key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, clean(tt.input))
		})
	}
}
