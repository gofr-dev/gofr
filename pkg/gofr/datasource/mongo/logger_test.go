package mongo

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggingDataPresent(t *testing.T) {
	queryLog := QueryLog{
		Query:      "find",
		Duration:   12345,
		Collection: "users",
		Filter:     map[string]string{"name": "John"},
		ID:         "123",
		Update:     map[string]string{"$set": "Doe"},
	}
	expected := "name:John"

	var buf bytes.Buffer

	queryLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
}

func TestLoggingEmptyData(t *testing.T) {
	queryLog := QueryLog{
		Query:    "insert",
		Duration: 6789,
	}
	expected := "name:John"

	var buf bytes.Buffer

	queryLog.PrettyPrint(&buf)

	assert.NotContains(t, buf.String(), expected)
}
