package response

import (
	"html/template"
	"io"
)

type Template struct { // Named as such to avoid conflict with imported template
	Data any
	Name string
}

func (t *Template) Render(w io.Writer) {
	tmpl := template.Must(template.ParseFiles("./templates/" + t.Name))
	_ = tmpl.Execute(w, t.Data)
}
