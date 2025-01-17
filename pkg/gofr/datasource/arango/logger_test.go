package arango

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_PrettyPrint(t *testing.T) {
	queryLog := QueryLog{
		Query:      "getDocument",
		Duration:   12345,
		Collection: "test",
	}
	expected := "getDocument"

	var buf bytes.Buffer

	queryLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
}
