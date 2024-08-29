package sftp

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileLogPrettyPrint(t *testing.T) {
	fileLog := FileLog{
		Operation: "Create file",
		Duration:  1234,
		Location:  "/ftp/one",
	}

	expected := "Create file"

	var buf bytes.Buffer

	fileLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
}

func TestFileLogPrettyPrintWhitespaceHandling(t *testing.T) {
	fileLog := FileLog{
		Operation: "  Create   file  ",
		Duration:  5678,
	}

	expectedMsg := "Create file"

	var buf bytes.Buffer

	fileLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expectedMsg)
}
