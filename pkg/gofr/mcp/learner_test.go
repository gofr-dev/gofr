package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestLearner_RegisterAndRecord(t *testing.T) {
	l := NewLearner("")

	l.Register("GET", "/users/{id}", []string{"id"})
	l.RecordBind("GET", "/users/{id}", reflect.TypeFor[testUser]())
	l.RecordQueryKey("GET", "/users/{id}", "include")
	l.RecordQueryKey("GET", "/users/{id}", "include") // dedup

	got := l.Schemas()
	rs, ok := got["GET /users/{id}"]
	require.True(t, ok, "expected schema for registered route")
	assert.Equal(t, []string{"id"}, rs.PathParams)
	assert.Equal(t, []string{"include"}, rs.QueryKeys)
	assert.Equal(t, "mcp.testUser", rs.BodyType)
	assert.Equal(t, "object", rs.Body["type"])
}

func TestLearner_RecordOnUnregisteredRoute_NoOp(t *testing.T) {
	l := NewLearner("")

	// No Register first — recording must not create entries.
	l.RecordBind("GET", "/nope", reflect.TypeFor[testUser]())
	l.RecordQueryKey("GET", "/nope", "x")

	assert.Empty(t, l.Schemas())
}

func TestLearner_RecordReturn_StripsPointer(t *testing.T) {
	l := NewLearner("")
	l.Register("GET", "/users/{id}", []string{"id"})

	u := &testUser{ID: "1", Name: "Ada"}
	l.RecordReturn("GET", "/users/{id}", u)

	rs := l.Schemas()["GET /users/{id}"]
	assert.Equal(t, "object", rs.Output["type"])
	props, _ := rs.Output["properties"].(map[string]any)
	assert.Contains(t, props, "id")
	assert.Contains(t, props, "name")
}

func TestLearner_PersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schemas.json")

	// Round 1: write.
	{
		l := NewLearner(path)
		l.Register("POST", "/users", nil)
		l.RecordBind("POST", "/users", reflect.TypeFor[testUser]())
		require.NoError(t, l.Save())
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "POST /users")

	// Round 2: a fresh learner reads the persisted file.
	{
		l := NewLearner(path)
		got := l.Schemas()
		rs, ok := got["POST /users"]
		require.True(t, ok, "expected schema to load from disk")
		assert.Equal(t, "POST", rs.Method)
		assert.Equal(t, "/users", rs.Path)
	}
}

func TestLearner_Save_NoPersist_NoOp(t *testing.T) {
	l := NewLearner("")
	l.Register("GET", "/x", nil)
	require.NoError(t, l.Save())
}
