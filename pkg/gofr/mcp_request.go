package gofr

import (
	"reflect"

	"gofr.dev/pkg/gofr/mcp"
)

// learningRequest wraps a Request so that the MCP layer can sharpen
// tool schemas based on actual handler usage. It is constructed only
// when MCP_ENABLED is set; otherwise the original Request flows through
// untouched.
//
// The wrapper is intentionally tiny: it forwards every method to the
// underlying Request and side-effects into the learner on Bind/Param.
// We do not record path-param reads because path params are already
// known statically from the mux route template.
type learningRequest struct {
	Request

	learner *mcp.Learner
	method  string
	path    string
}

func newLearningRequest(req Request, learner *mcp.Learner, method, path string) Request {
	if learner == nil {
		return req
	}

	return &learningRequest{
		Request: req,
		learner: learner,
		method:  method,
		path:    path,
	}
}

// Bind records the bound type so the MCP tool's input schema can be
// generated from the handler's actual struct. The underlying Bind is
// always called; we only learn on success so partially-decoded JSON
// doesn't pollute the schema.
func (l *learningRequest) Bind(i any) error {
	err := l.Request.Bind(i)
	if err != nil || i == nil {
		return err
	}

	t := reflect.TypeOf(i)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t != nil {
		l.learner.RecordBind(l.method, l.path, t)
	}

	return nil
}

// Param records the query key the handler reads, then returns the
// value from the underlying Request unchanged.
func (l *learningRequest) Param(key string) string {
	l.learner.RecordQueryKey(l.method, l.path, key)

	return l.Request.Param(key)
}

// Params records the query key and returns the values unchanged.
func (l *learningRequest) Params(key string) []string {
	l.learner.RecordQueryKey(l.method, l.path, key)

	return l.Request.Params(key)
}
