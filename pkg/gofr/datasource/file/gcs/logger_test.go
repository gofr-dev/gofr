package gcs

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileLogPrettyPrint(t *testing.T) {
	msg := "File Created successfully"

	fileLog := FileLog{
		Operation: "Create file",
		Duration:  1234,
		Location:  "/ftp/one",
		Message:   &msg,
	}

	expected := "Create file"

	expectedMsg := "File Created successfully"

	var buf bytes.Buffer

	fileLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
	assert.Contains(t, buf.String(), expectedMsg)
}

func TestFileLogPrettyPrintWhitespaceHandling(t *testing.T) {
	msg := "  File   creation    complete  "
	fileLog := FileLog{
		Operation: "  Create   file  ",
		Duration:  5678,
		Message:   &msg,
	}
	expected := "Create file"
	expectedMsg := "File creation complete"

	var buf bytes.Buffer

	fileLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
	assert.Contains(t, buf.String(), expectedMsg)
}
