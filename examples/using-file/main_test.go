package main

import (
	"testing"

	"gofr.dev/pkg/gofr/assert"
)

// TestIntegration to test the behavior of main function
func TestIntegration(t *testing.T) {
	testCases := []struct {
		command  string
		expected string
	}{
		{"cmd write", "File written successfully!"},
		{"cmd read", "Welcome to gofr.dev!"},
		{"cmd list", "Readme.md configs handler main.go main_test.go test.txt"},
		{"cmd move -src=test.txt -dest=test1.txt", "File moved successfully"},
		{"cmd move -dest=NewDir/test.txt", "Parameter src is required for this request"},
		{"cmd move -src=test.txt", "Parameter dest is required for this request"},
		{"cmd randomCommand", "No Command Found!"},
		{"cmd", "No Command Found!"},
	}

	for _, tc := range testCases {
		assert.CMDOutputContains(t, main, tc.command, tc.expected)
	}
}
