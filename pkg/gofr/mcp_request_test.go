package gofr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/mcp"
)

// stubRequest implements gofr.Request and lets tests inject what
// Bind/Param/Params return so we can verify the wrapper's recording
// without standing up a full HTTP server.
type stubRequest struct {
	bound       any
	bindErr     error
	paramVal    string
	paramsVal   []string
	pathParam   string
	hostName    string
	ctx         context.Context
	bindCalls   int
	paramKeys   []string
	paramsKeys  []string
}

func (s *stubRequest) Context() context.Context { return s.ctx }
func (s *stubRequest) HostName() string         { return s.hostName }
func (s *stubRequest) PathParam(string) string  { return s.pathParam }

func (s *stubRequest) Param(key string) string {
	s.paramKeys = append(s.paramKeys, key)
	return s.paramVal
}

func (s *stubRequest) Params(key string) []string {
	s.paramsKeys = append(s.paramsKeys, key)
	return s.paramsVal
}

func (s *stubRequest) Bind(i any) error {
	s.bindCalls++

	if s.bindErr != nil {
		return s.bindErr
	}

	if u, ok := i.(*sampleBindUser); ok {
		*u = sampleBindUser{ID: "1", Name: "Ada"}
	}

	s.bound = i

	return nil
}

type sampleBindUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestLearningRequest_NilLearnerReturnsOriginal(t *testing.T) {
	stub := &stubRequest{}
	got := newLearningRequest(stub, nil, "GET", "/x")
	assert.Same(t, stub, got)
}

func TestLearningRequest_RecordsBindOnSuccess(t *testing.T) {
	l := mcp.NewLearner("")
	l.Register("POST", "/users", nil)

	stub := &stubRequest{}
	wrapped := newLearningRequest(stub, l, "POST", "/users")

	var u sampleBindUser
	require.NoError(t, wrapped.Bind(&u))

	rs := l.Schemas()["POST /users"]
	assert.Equal(t, "gofr.sampleBindUser", rs.BodyType)
	assert.NotEmpty(t, rs.Body)
}

func TestLearningRequest_DoesNotRecordOnBindError(t *testing.T) {
	l := mcp.NewLearner("")
	l.Register("POST", "/users", nil)

	stub := &stubRequest{bindErr: assertAnError}
	wrapped := newLearningRequest(stub, l, "POST", "/users")

	var u sampleBindUser
	err := wrapped.Bind(&u)
	require.Error(t, err)

	rs := l.Schemas()["POST /users"]
	assert.Empty(t, rs.BodyType, "failed binds must not pollute the schema")
}

func TestLearningRequest_RecordsParam(t *testing.T) {
	l := mcp.NewLearner("")
	l.Register("GET", "/users", nil)

	stub := &stubRequest{paramVal: "10"}
	wrapped := newLearningRequest(stub, l, "GET", "/users")

	_ = wrapped.Param("limit")
	_ = wrapped.Param("offset")
	_ = wrapped.Params("tag")

	rs := l.Schemas()["GET /users"]
	assert.Contains(t, rs.QueryKeys, "limit")
	assert.Contains(t, rs.QueryKeys, "offset")
	assert.Contains(t, rs.QueryKeys, "tag")
}

// assertAnError is just any non-nil error for the bind-error path.
var assertAnError = errSentinel("bind failed")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
