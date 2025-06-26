package clickhouse

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggingDataPresent(t *testing.T) {
	queryLog := Log{
		Type:     "SELECT",
		Query:    "SELECT * FROM users",
		Duration: 12345,
	}
	expected := "SELECT"

	var buf bytes.Buffer

	queryLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
}
