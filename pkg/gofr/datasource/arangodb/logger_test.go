package arangodb

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PrettyPrint(t *testing.T) {
	queryLog := QueryLog{
		Query:      "",
		Duration:   12345,
		Database:   "test",
		Collection: "test",
		Filter:     true,
		ID:         "12345",
		Operation:  "getDocument",
	}
	expected := "getDocument"

	var buf bytes.Buffer

	queryLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
}
