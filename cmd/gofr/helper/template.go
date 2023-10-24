package helper

import (
	"bytes"
	"text/template"
)

type Help struct {
	Example     string
	Flag        string
	Usage       string
	Description string
}

// Generate formats help information based on the provided Help struct
func Generate(value Help) string {
	templateStr := `{{.Description}}

usage: {{.Usage}}

Flag:
{{ .Flag }}

Examples:
{{ .Example }}

`

	temp := template.Must(template.New("help_template").Parse(templateStr))

	var resultBytes bytes.Buffer

	err := temp.Execute(&resultBytes, value)
	if err != nil {
		return ""
	}

	return resultBytes.String()
}
