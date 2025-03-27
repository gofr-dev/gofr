package response

import (
	"html/template"
	"io"
	"net/http"
)

type Template struct { // Named as such to avoid conflict with imported template
	Cookie  *http.Cookie
	Headers map[string]string
	Data    any
	Name    string
}

func (t *Template) Render(w io.Writer) {
	tmpl := template.Must(template.ParseFiles("./templates/" + t.Name))
	_ = tmpl.Execute(w, t.Data)
}
