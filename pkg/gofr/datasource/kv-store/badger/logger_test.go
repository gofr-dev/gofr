package badger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PrettyPrint(t *testing.T) {
	queryLog := Log{
		Type:     "GET",
		Duration: 12345,
	}
	expected := "GET"

	var buf bytes.Buffer

	queryLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
}
