package cmd

import (
	"context"
	"reflect"
	"strconv"
)

// Request is an abstraction over the actual command with flags. This abstraction is useful because it allows us
// to create cmd applications in same way we would create a HTTP server application.
// Gofr's http.Request is another such abstraction.
type Request struct {
	flags  map[string]bool
	params map[string]string
}

// TODO - use statement to parse the request to populate the flags and params.

// NewRequest creates a Request from a list of arguments. This way we can simulate running a command without actually
// doing it. It makes the code more testable this way.
func NewRequest(args []string) *Request {
	r := Request{
		flags:  make(map[string]bool),
		params: make(map[string]string),
	}

	return &r
}

// Param returns the value of the parameter for key.
func (r *Request) Param(key string) string {
	return r.params[key]
}

// PathParam returns the value of the parameter for key. This is equivalent to Param.
func (r *Request) PathParam(key string) string {
	return r.params[key]
}

func (r *Request) Context() context.Context {
	return context.Background()
}

func (r *Request) Bind(i interface{}) error {
	// pointer to struct - addressable
	ps := reflect.ValueOf(i)
	// struct
	s := ps.Elem()
	if s.Kind() == reflect.Struct {
		for k, v := range r.params {
			f := s.FieldByName(k)
			// A Value can be changed only if it is addressable and not unexported struct field
			if f.IsValid() && f.CanSet() {
				//nolint:exhaustive // no need to add other cases
				switch f.Kind() {
				case reflect.String:
					f.SetString(v)
				case reflect.Bool:
					if v == "true" {
						f.SetBool(true)
					}
				case reflect.Int:
					n, _ := strconv.Atoi(v)
					f.SetInt(int64(n))
				}
			}
		}
	}

	return nil
}
