package oracle

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
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
