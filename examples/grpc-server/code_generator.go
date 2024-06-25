package main

import (
	"bytes"
	"go/format"
	"text/template"
)

// GenerateCode generates the client and server code
func GenerateCode(parsedInterface *ParsedInterface) (string, string, error) {
	clientTemplate := `
package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
)

type {{.InterfaceName}}Client struct {
	cc *grpc.ClientConn
}

func New{{.InterfaceName}}Client(cc *grpc.ClientConn) *{{.InterfaceName}}Client {
	return &{{.InterfaceName}}Client{cc: cc}
}

{{range .Methods}}
func (c *{{$.InterfaceName}}Client) {{.Name}}(ctx context.Context, req interface{}) (interface{}, error) {
	out := new(interface{})
	err := c.cc.Invoke(ctx, "/{{$.InterfaceName}}/{{.Name}}", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
{{end}}
`

	serverTemplate := `
package main

import (
	"context"
	"fmt"
	"reflect"
	"google.golang.org/grpc"
)

type {{.InterfaceName}}Server struct {
	impl interface{}
}

func New{{.InterfaceName}}Server(impl interface{}) *{{.InterfaceName}}Server {
	return &{{.InterfaceName}}Server{impl: impl}
}

{{range .Methods}}
func (s *{{$.InterfaceName}}Server) {{.Name}}(ctx context.Context, req struct {
	{{range .InputParams}}{{.Name}} {{.Type}}
	{{end}}
}) ({{range .ReturnTypes}}{{.Name}} {{.Type}}{{end}}, error) {
	method := reflect.ValueOf(s.impl).MethodByName("{{.Name}}")
	if !method.IsValid() {
		return {{range .ReturnTypes}}{{.Name}}, fmt.Errorf("method {{.Name}} not found")
	}
	in := []reflect.Value{reflect.ValueOf(ctx)}
	{{range .InputParams}}in = append(in, reflect.ValueOf(req.{{.Name}})){{end}}
	out := method.Call(in)
	if len(out) != {{len .ReturnTypes}}+1 {
		return {{range .ReturnTypes}}{{.Name}}, fmt.Errorf("unexpected number of return values")
	}
	if err, ok := out[{{len .ReturnTypes}}].Interface().(error); ok && err != nil {
		return {{range .ReturnTypes}}{{.Name}}, err
	}
	resp := struct {
		{{range .ReturnTypes}}
			{{.Name}} {{.Type}}
		{{end}}
	}{}
	{{range $i, $type := .ReturnTypes}}
		{{if $type.Fields}}
			resp.{{.Name}} = {{.Type}}{
				{{range $field := $type.Fields}}
					{{$field.Name}}: out[{{$i}}].Elem().FieldByName("{{$field.Name}}").Interface().({{$field.Type}}),
				{{end}}
			}
		{{else}}
			resp.{{.Name}} = out[{{$i}}].Interface().({{$type.Type}})
		{{end}}
	{{end}}
	return resp, nil
}
{{end}}

func (s *{{.InterfaceName}}Server) Register(grpcServer *grpc.Server) {
	{{.InterfaceName}}_RegisterService(grpcServer, s.impl)
}
`

	clientTmpl, err := template.New("client").Parse(clientTemplate)
	if err != nil {
		return "", "", err
	}

	serverTmpl, err := template.New("server").Parse(serverTemplate)
	if err != nil {
		return "", "", err
	}

	var clientBuffer, serverBuffer bytes.Buffer

	err = clientTmpl.Execute(&clientBuffer, parsedInterface)
	if err != nil {
		return "", "", err
	}

	err = serverTmpl.Execute(&serverBuffer, parsedInterface)
	if err != nil {
		return "", "", err
	}

	clientCode, err := format.Source(clientBuffer.Bytes())
	if err != nil {
		return "", "", err
	}

	serverCode, err := format.Source(serverBuffer.Bytes())
	if err != nil {
		return string(clientCode), "", err
	}

	return string(clientCode), string(serverCode), nil
}
