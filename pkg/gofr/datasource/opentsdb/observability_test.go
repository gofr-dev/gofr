package opentsdb

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

func TestClean(t *testing.T) {
	query := "  select   *\n from\tmetrics  "

	tests := []struct {
		desc  string
		query *string
		exp   string
	}{
		{desc: "nil query", query: nil, exp: ""},
		{desc: "collapses whitespace", query: &query, exp: "select * from metrics"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.exp, clean(tc.query))
		})
	}
}

func TestQueryLog_PrettyPrint(t *testing.T) {
	status := statusSuccess
	message := "query   done"

	tests := []struct {
		desc        string
		log         QueryLog
		expContains []string
	}{
		{
			desc:        "with status and message",
			log:         QueryLog{Operation: "Query", Duration: 42, Status: &status, Message: &message},
			expContains: []string{"Query", "OPENTSDB", "42", statusSuccess, "query done"},
		},
		{
			desc:        "without status and message",
			log:         QueryLog{Operation: "HealthCheck", Duration: 7},
			expContains: []string{"HealthCheck", "OPENTSDB", "7"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer

			tc.log.PrettyPrint(&buf)

			for _, s := range tc.expContains {
				assert.Contains(t, buf.String(), s)
			}
		})
	}
}

func TestAddTracer_NilTracerReturnsNilSpan(t *testing.T) {
	var tracer trace.Tracer

	assert.Nil(t, addTracer(t.Context(), tracer, "Query", "QueryResponse"))
}
