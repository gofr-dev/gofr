package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ErrCommandNotFound(t *testing.T) {
	err := ErrCommandNotFound{}

	result := err.Error()

	assert.Equal(t, "No Command Found!", result, "Test Failed \nMatch Error String")
}

func Test_addRouteNotNilHandler(t *testing.T) {
	testHandler := func(c *Context) (interface{}, error) {
		return nil, nil
	}

	testCases := []struct {
		desc    string
		pattern string
		handler func(c *Context) (interface{}, error)
	}{
		{"valid pattern and handler", "test", testHandler},
		{"empty pattern and valid handler", " ", testHandler},
	}

	for i, tc := range testCases {
		cmd := &cmd{}

		cmd.addRoute(tc.pattern, tc.handler)

		assert.Equal(t, tc.pattern, cmd.routes[0].pattern, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.NotNil(t, cmd.routes[0].handler, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_addRouteNilHanlder(t *testing.T) {
	testCases := []struct {
		desc    string
		pattern string
		handler func(c *Context) (interface{}, error)
	}{
		{"valid pattern and nil handler", "test", nil},
		{"empty pattern and nil handler", " ", nil},
	}

	for i, tc := range testCases {
		cmd := &cmd{}

		cmd.addRoute(tc.pattern, tc.handler)

		assert.Equal(t, tc.pattern, cmd.routes[0].pattern, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Nil(t, cmd.routes[0].handler, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_handlerReturnsNilIfRouteHandlerIsNotNil(t *testing.T) {
	cmd := &cmd{
		routes: []route{
			{pattern: "pattern", handler: func(c *Context) (interface{}, error) {
				return nil, nil
			}},
		},
	}
	path := "pattern"

	result := cmd.handler(path)

	assert.NotNil(t, result, "TEST, Failed.\n not nil handler")
}

func Test_handlerReturnsNil(t *testing.T) {
	nonMatchingPattern := &cmd{
		routes: []route{
			{pattern: "pattern1", handler: func(c *Context) (interface{}, error) {
				return nil, nil
			}},
			{pattern: "pattern2", handler: func(c *Context) (interface{}, error) {
				return nil, nil
			}},
		},
	}

	emptyRouteSlice := &cmd{routes: []route{}}

	nilHandler := &cmd{
		routes: []route{
			{pattern: "pattern", handler: nil},
		},
	}

	testCases := []struct {
		desc    string
		cmd     cmd
		pattern string
	}{
		{"pattern not matching with routes", *nonMatchingPattern, "non-matching-pattern"},
		{"routes slice in empty", *emptyRouteSlice, "pattern"},
		{"handler in nil", *nilHandler, "pattern"},
	}

	for i, tc := range testCases {
		r := tc.cmd.handler(tc.pattern)

		assert.Nil(t, r, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
