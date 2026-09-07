package response

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// templatesDir is the directory Render reads from (it parses "./templates/<name>").
const templatesDir = "templates"

// createTemplate writes a template file under the templates directory and registers
// cleanup of the whole directory.
func createTemplate(t *testing.T, name, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(templatesDir, os.ModePerm))

	require.NoError(t, os.WriteFile(filepath.Join(templatesDir, name), []byte(content), 0600))

	t.Cleanup(func() {
		_ = os.RemoveAll(templatesDir)
	})
}

func TestTemplate_Render(t *testing.T) {
	tests := []struct {
		desc     string
		name     string
		content  string
		data     any
		expected string
	}{
		{
			desc:     "renders fields from struct-like map",
			name:     "page.html",
			content:  `<title>{{.Title}}</title>`,
			data:     map[string]string{"Title": "Hello"},
			expected: `<title>Hello</title>`,
		},
		{
			desc:     "static template without data",
			name:     "static.html",
			content:  `<p>static</p>`,
			data:     nil,
			expected: `<p>static</p>`,
		},
		{
			desc:     "escapes html in data",
			name:     "escape.html",
			content:  `<p>{{.Body}}</p>`,
			data:     map[string]string{"Body": "<script>"},
			expected: `<p>&lt;script&gt;</p>`,
		},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			createTemplate(t, tc.name, tc.content)

			var buf bytes.Buffer

			tmpl := &Template{Name: tc.name, Data: tc.data}

			tmpl.Render(&buf)

			assert.Equal(t, tc.expected, buf.String(), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestTemplate_Render_MissingFilePanics(t *testing.T) {
	// Render uses template.Must, which panics when the file cannot be parsed/found.
	tmpl := &Template{Name: "does-not-exist.html", Data: nil}

	assert.Panics(t, func() {
		tmpl.Render(&bytes.Buffer{})
	})
}
