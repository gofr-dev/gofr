package main

import (
	"testing"

	"gofr.dev/pkg/gofr/assert"
)

func Test_Main(t *testing.T) {
	openAPIData := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
</head>
<body>
{{range .Items}}<div>{{ . }}</div>{{else}}<div><strong>no rows</strong></div>{{end}}
</body>
</html>`
	testCases := []struct {
		command  string
		expected string
	}{
		{"cmd hello", "Hello!"},
		{"cmd hello -name=Vikash", "Hello Vikash!"},

		{"cmd error", "some error occurred"},

		{"cmd bind", "Name:  Good: false"},
		{"cmd bind -Name=Vikash", "Name: Vikash Good: false"},
		{"cmd bind -Name=Vikash -IsGood", "Name: Vikash Good: true"},
		{"cmd bind -Name=Hen -IsGood=false", "Name: Hen Good: false"},

		{"cmd unknown", "No Command Found!"},

		{"cmd file", "Hello"},
		{"cmd temp -filename=sample.html", openAPIData},
		{"cmd temp -filename=incorrect.txt", "File incorrect.txt not found at location templates"},
	}

	for _, tc := range testCases {
		assert.CMDOutputContains(t, main, tc.command, tc.expected)
	}
}
