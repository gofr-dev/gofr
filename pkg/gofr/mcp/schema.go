// Package mcp turns a running GoFr application into a Model Context
// Protocol (MCP) server when MCP_ENABLED is set. The implementation is
// deliberately small: every existing route runs through the same
// middleware chain regardless of whether the call arrived as HTTP or
// MCP, so auth, RBAC, observability, and rate limiting are inherited
// for free. See pkg/gofr/mcp/server.go for the dispatcher.
package mcp

import (
	"maps"
	"reflect"
	"strings"
	"time"
)

// Schema is a JSON-Schema-shaped map. We emit Draft-7-compatible shapes
// — the lowest common denominator that today's MCP clients accept.
type Schema map[string]any

var timeType = reflect.TypeFor[time.Time]()

// FromType walks t and produces a JSON Schema describing the values t
// can take when JSON-encoded. The point is to give an LLM enough
// structure to construct valid tool inputs from the actual Go type
// the handler binds, instead of a hand-maintained spec.
//
// Unsupported kinds (chan, func, unsafe.Pointer) collapse to a
// schemaless "object" so the LLM sees something rather than nothing.
func FromType(t reflect.Type) Schema {
	if t == nil {
		return Schema{}
	}

	return fromType(t, map[reflect.Type]bool{})
}

func fromType(t reflect.Type, seen map[reflect.Type]bool) Schema {
	if seen[t] {
		// Recursive type — break the cycle. Without this a linked-list
		// or tree type would blow the stack.
		return Schema{"type": "object"}
	}

	switch t.Kind() {
	case reflect.Pointer:
		return fromType(t.Elem(), seen)

	case reflect.String:
		return Schema{"type": "string"}

	case reflect.Bool:
		return Schema{"type": "boolean"}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Schema{"type": "integer"}

	case reflect.Float32, reflect.Float64:
		return Schema{"type": "number"}

	case reflect.Slice, reflect.Array:
		// []byte is JSON-encoded as base64; everything else is an array.
		if t.Elem().Kind() == reflect.Uint8 {
			return Schema{"type": "string", "contentEncoding": "base64"}
		}

		return Schema{"type": "array", "items": fromType(t.Elem(), seen)}

	case reflect.Map:
		// JSON object keys must be strings — anything else collapses
		// to a schemaless object rather than emitting an invalid schema.
		if t.Key().Kind() != reflect.String {
			return Schema{"type": "object"}
		}

		return Schema{
			"type":                 "object",
			"additionalProperties": fromType(t.Elem(), seen),
		}

	case reflect.Interface:
		// Empty interface accepts anything; let the LLM send anything.
		return Schema{}

	case reflect.Struct:
		if t == timeType {
			return Schema{"type": "string", "format": "date-time"}
		}

		return structSchema(t, seen)

	default:
		// chan, func, unsafe.Pointer — JSON can't carry these.
		return Schema{"type": "object"}
	}
}

func structSchema(t reflect.Type, seen map[reflect.Type]bool) Schema {
	seen[t] = true
	defer delete(seen, t)

	props := map[string]any{}

	var required []string

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}

		// Anonymous struct without a JSON tag: encoding/json promotes
		// its fields to the parent. We mirror that so the schema lines
		// up with what JSON unmarshal will actually accept. Note: this
		// runs even when the embedded type itself is unexported — Go
		// still promotes the inner *exported* fields in that case.
		if f.Anonymous && tag == "" && isStructLike(f.Type) {
			inner := f.Type
			if inner.Kind() == reflect.Pointer {
				inner = inner.Elem()
			}

			embedded := structSchema(inner, seen)
			if embedProps, ok := embedded["properties"].(map[string]any); ok {
				maps.Copy(props, embedProps)
			}

			if embedReq, ok := embedded["required"].([]string); ok {
				required = append(required, embedReq...)
			}

			continue
		}

		// Non-embedded unexported fields are invisible to encoding/json.
		if !f.IsExported() {
			continue
		}

		name, omitempty := parseJSONTag(tag, f.Name)

		props[name] = fromType(f.Type, seen)

		// Pointer fields and omitempty fields are optional. Everything
		// else is required — that matches how the handler will actually
		// behave under encoding/json (zero values are accepted but the
		// keys must be present for the schema to validate strictly).
		if !omitempty && f.Type.Kind() != reflect.Pointer {
			required = append(required, name)
		}
	}

	s := Schema{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}

	return s
}

func isStructLike(t reflect.Type) bool {
	if t.Kind() == reflect.Struct {
		return true
	}

	return t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct
}

// parseJSONTag returns (name, omitempty). An empty tag value means
// "use the Go field name."
func parseJSONTag(tag, fieldName string) (string, bool) {
	if tag == "" {
		return fieldName, false
	}

	parts := strings.Split(tag, ",")

	name := parts[0]
	if name == "" {
		name = fieldName
	}

	omitempty := false

	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
		}
	}

	return name, omitempty
}
