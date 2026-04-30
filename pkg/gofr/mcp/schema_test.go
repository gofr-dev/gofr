package mcp

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type sampleUser struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Age       int       `json:"age,omitempty"`
	Internal  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	notSeen   string    //nolint:unused // verifies unexported fields stay out of the schema
}

type sampleAddress struct {
	Line1 string `json:"line1"`
	City  string `json:"city,omitempty"`
}

type sampleEmbedded struct {
	sampleAddress
	Tag *string `json:"tag,omitempty"`
}

type sampleNested struct {
	User    sampleUser     `json:"user"`
	Friends []sampleUser   `json:"friends"`
	Lookup  map[string]int `json:"lookup"`
}

// linked covers the recursion guard.
type linked struct {
	Value int     `json:"value"`
	Next  *linked `json:"next,omitempty"`
}

func TestFromType_Primitives(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want Schema
	}{
		{"string", "", Schema{"type": "string"}},
		{"bool", true, Schema{"type": "boolean"}},
		{"int", 0, Schema{"type": "integer"}},
		{"int64", int64(0), Schema{"type": "integer"}},
		{"uint32", uint32(0), Schema{"type": "integer"}},
		{"float64", 0.0, Schema{"type": "number"}},
		{"time", time.Time{}, Schema{"type": "string", "format": "date-time"}},
		{"bytes", []byte{}, Schema{"type": "string", "contentEncoding": "base64"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, FromType(reflect.TypeOf(c.in)))
		})
	}
}

func TestFromType_Struct_RespectsJSONTags(t *testing.T) {
	got := FromType(reflect.TypeOf(sampleUser{}))

	assert.Equal(t, "object", got["type"])

	props, _ := got["properties"].(map[string]any)
	assert.Contains(t, props, "id")
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "age")
	assert.Contains(t, props, "created_at")
	assert.NotContains(t, props, "Internal")
	assert.NotContains(t, props, "notSeen")

	required, _ := got["required"].([]string)
	assert.Contains(t, required, "id")
	assert.Contains(t, required, "name")
	assert.NotContains(t, required, "age", "omitempty fields must not be required")
}

func TestFromType_Pointer_NotRequired(t *testing.T) {
	got := FromType(reflect.TypeOf(sampleEmbedded{}))

	required, _ := got["required"].([]string)
	assert.NotContains(t, required, "tag", "pointer fields should be optional")
}

func TestFromType_Embedded_Flattened(t *testing.T) {
	got := FromType(reflect.TypeOf(sampleEmbedded{}))

	props, _ := got["properties"].(map[string]any)
	assert.Contains(t, props, "line1", "anonymous struct without tag must flatten to parent properties")
	assert.Contains(t, props, "city")
	assert.Contains(t, props, "tag")
}

func TestFromType_Slice_Map_Nested(t *testing.T) {
	got := FromType(reflect.TypeOf(sampleNested{}))

	props, _ := got["properties"].(map[string]any)

	friends, _ := props["friends"].(Schema)
	assert.Equal(t, "array", friends["type"])
	items, _ := friends["items"].(Schema)
	assert.Equal(t, "object", items["type"])

	lookup, _ := props["lookup"].(Schema)
	assert.Equal(t, "object", lookup["type"])
	addl, _ := lookup["additionalProperties"].(Schema)
	assert.Equal(t, "integer", addl["type"])
}

func TestFromType_Recursive_DoesNotBlowStack(t *testing.T) {
	// Without the seen-set guard this recurses forever.
	got := FromType(reflect.TypeOf(linked{}))

	props, _ := got["properties"].(map[string]any)
	next, _ := props["next"].(Schema)
	assert.Equal(t, "object", next["type"], "cycle should collapse to schemaless object")
}

func TestFromType_NilType_ReturnsEmptySchema(t *testing.T) {
	assert.Equal(t, Schema{}, FromType(nil))
}

func TestFromType_Interface_Empty(t *testing.T) {
	type holder struct {
		Anything any `json:"anything"`
	}

	got := FromType(reflect.TypeOf(holder{}))
	props, _ := got["properties"].(map[string]any)
	any1, _ := props["anything"].(Schema)
	assert.Empty(t, any1, "interface fields should accept anything")
}

func TestParseJSONTag(t *testing.T) {
	cases := []struct {
		tag       string
		field     string
		wantName  string
		wantOmit  bool
	}{
		{"", "Name", "Name", false},
		{"name", "Name", "name", false},
		{"name,omitempty", "Name", "name", true},
		{",omitempty", "Name", "Name", true},
	}

	for _, c := range cases {
		gotName, gotOmit := parseJSONTag(c.tag, c.field)
		assert.Equal(t, c.wantName, gotName, "tag=%q", c.tag)
		assert.Equal(t, c.wantOmit, gotOmit, "tag=%q", c.tag)
	}
}
