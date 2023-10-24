// Package request provides the implementation of operations related to a http and cmd request
// it gives the features like reading parameters, bind request body, etc.
package request

import (
	"flag"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// CMD (Command) is a data structure designed to access and manage various types of parameters,
// including path parameters, query parameters, and request bodies, used in command-line (cmd) applications.
//
// It provides a simple and structured way to work with input parameters and data, making it easier to
// handle and process command-line input.
type CMD struct {
	// contains exported fields
	params map[string]string
}

// NewCMDRequest creates a new CMD request instance.
func NewCMDRequest() Request {
	c := &CMD{}

	flag.Parse()
	args := flag.Args()
	c.parseArgs(args)

	return c
}

func (c *CMD) parseArgs(args []string) {
	c.params = make(map[string]string)

	const (
		argsLen1 = 1
		argsLen2 = 2
	)

	for _, arg := range args {
		if arg[0] != '-' {
			continue
		}

		a := arg[1:]

		switch values := strings.Split(a, "="); len(values) {
		case argsLen1:
			// Support -t -a etc.
			c.params[values[0]] = "true"
		case argsLen2:
			// Support -a=b
			c.params[values[0]] = values[1]
		}
	}
}

// Param retrieves a parameter by key.
func (c *CMD) Param(key string) string {
	return c.params[key]
}

// PathParam retrieves a parameter by key (same as Param).
func (c *CMD) PathParam(key string) string {
	return c.params[key]
}

// Header retrieves a parameter by key (same as Param).
func (c *CMD) Header(key string) string {
	return c.Param(key)
}

// Params returns all parameters as a map.
func (c *CMD) Params() map[string]string {
	return c.params
}

// Request returns a nil HTTP request.
func (c *CMD) Request() *http.Request {
	return nil
}

// Bind binds parameters to a struct.
//
//nolint:gocognit // Reducing cognitive complexity will make it harder to read.
func (c *CMD) Bind(i interface{}) error {
	// pointer to struct - addressable
	ps := reflect.ValueOf(i)
	// struct
	s := ps.Elem()
	if s.Kind() == reflect.Struct {
		for k, v := range c.params {
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

// BindStrict is an alias for Bind.
func (c *CMD) BindStrict(i interface{}) error {
	return c.Bind(i)
}

// GetClaims returns nil claims for every request
func (c *CMD) GetClaims() map[string]interface{} {
	return nil
}

// GetClaim returns nil claim value for every request
func (c *CMD) GetClaim(string) interface{} {
	return nil
}
