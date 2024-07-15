package cmd

import (
	"context"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Request is an abstraction over the actual command with flags. This abstraction is useful because it allows us
// to create cmd applications in same way we would create a HTTP server application.
// Gofr's http.Request is another such abstraction.
type Request struct {
	flags  map[string]bool
	params map[string]string
}

const trueString = "true"

// TODO - use statement to parse the request to populate the flags and params.

// NewRequest creates a Request from a list of arguments. This way we can simulate running a command without actually
// doing it. It makes the code more testable this way.
func NewRequest(args []string) *Request {
	r := Request{
		flags:  make(map[string]bool),
		params: make(map[string]string),
	}

	const (
		argsLen1 = 1
		argsLen2 = 2
	)

	for _, arg := range args {
		if arg == "" {
			continue // This takes cares of cases where command has multiple space in between.
		}

		if arg[0] != '-' {
			continue
		}

		if len(arg) == 1 {
			continue
		}

		a := ""
		if arg[1] == '-' {
			a = arg[2:]
		} else {
			a = arg[1:]
		}

		switch values := strings.Split(a, "="); len(values) {
		case argsLen1:
			// Support -t -a etc.
			r.params[values[0]] = trueString
		case argsLen2:
			// Support -a=b
			r.params[values[0]] = values[1]
		}
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

func (*Request) Context() context.Context {
	return context.Background()
}

func (*Request) HostName() (hostname string) {
	hostname, _ = os.Hostname()

	return hostname
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
					if v == trueString {
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
