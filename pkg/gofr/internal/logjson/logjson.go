// Package logjson writes the JSON of GoFr's own log lines without reflection. Every function here
// either produces exactly the bytes encoding/json produces for the same value, or reports false so
// the caller falls back to encoding/json. It is internal so none of this becomes public API.
package logjson

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// AppendFunc appends the JSON encoding of v to dst. It reports false, with dst unchanged in
// meaning, when v is not a type it handles or holds a value it does not encode byte-for-byte as
// encoding/json would. The caller then discards what was appended and uses encoding/json.
type AppendFunc func(dst []byte, v any) ([]byte, bool)

// appenders holds the registered message encoders. It is read on every log line and written only
// at setup, so readers take an atomic load and writers copy the slice under a mutex.
//
//nolint:gochecknoglobals // a message type encodes the same way whichever logger writes it.
var (
	appenders  atomic.Pointer[[]AppendFunc]
	registerMu sync.Mutex
)

// Register adds fn to the encoders AppendMessage tries. A package registers the encoder for a type
// it owns, so the logging package can write that type without importing it.
func Register(fn AppendFunc) {
	registerMu.Lock()
	defer registerMu.Unlock()

	var next []AppendFunc
	if cur := appenders.Load(); cur != nil {
		next = append(next, *cur...)
	}

	next = append(next, fn)
	appenders.Store(&next)
}

// AppendMessage appends the JSON of a log message: a string, or a value of a registered type.
func AppendMessage(dst []byte, v any) ([]byte, bool) {
	if s, ok := v.(string); ok {
		return AppendString(dst, s)
	}

	fns := appenders.Load()
	if fns == nil {
		return dst, false
	}

	for _, fn := range *fns {
		if out, ok := fn(dst, v); ok {
			return out, true
		}
	}

	return dst, false
}

// AppendString appends s as a JSON string, escaped as encoding/json escapes it with its default
// HTML escaping. Only printable ASCII is handled. A control byte, DEL or any non-ASCII byte reports
// false: those take escapes (\n, \u00XX, \u2028, U+FFFD for invalid UTF-8) whose exact spelling is
// left to encoding/json, so the output cannot drift between Go versions.
func AppendString(dst []byte, s string) ([]byte, bool) {
	dst = append(dst, '"')
	start := 0

	for i := 0; i < len(s); i++ {
		c := s[i]

		var esc string

		switch {
		case c == '"':
			esc = `\"`
		case c == '\\':
			esc = `\\`
		case c == '<':
			esc = `\u003c`
		case c == '>':
			esc = `\u003e`
		case c == '&':
			esc = `\u0026`
		case c < 0x20 || c >= 0x7f:
			return dst, false
		default:
			continue
		}

		dst = append(dst, s[start:i]...)
		dst = append(dst, esc...)
		start = i + 1
	}

	dst = append(dst, s[start:]...)

	return append(dst, '"'), true
}

// AppendInt appends n as encoding/json writes an integer.
func AppendInt(dst []byte, n int64) []byte {
	return strconv.AppendInt(dst, n, decimal)
}

const (
	decimal        = 10
	maxYear        = 9999
	secondsPerHour = 3600
	hoursPerDay    = 24
	secondsPerMin  = 60
)

// AppendTime appends t as encoding/json writes a time.Time: an RFC 3339 string with nanoseconds.
// encoding/json rejects a year outside [0,9999] or a zone offset of 24 hours or more, and RFC 3339
// cannot show an offset with seconds; all three report false.
func AppendTime(dst []byte, t time.Time) ([]byte, bool) {
	if y := t.Year(); y < 0 || y > maxYear {
		return dst, false
	}

	_, offset := t.Zone()
	if offset%secondsPerMin != 0 || offset >= hoursPerDay*secondsPerHour || offset <= -hoursPerDay*secondsPerHour {
		return dst, false
	}

	dst = append(dst, '"')
	dst = t.AppendFormat(dst, time.RFC3339Nano)

	return append(dst, '"'), true
}

// Object appends a JSON object field by field. Each field follows encoding/json's omitempty rule:
// an empty string or a zero integer is left out. Fields must be added in struct-field order, which
// is the order encoding/json writes them.
type Object struct {
	buf   []byte
	start int
	ok    bool
}

// BeginObject starts an object at the end of dst.
func BeginObject(dst []byte) Object {
	dst = append(dst, '{')

	return Object{buf: dst, start: len(dst), ok: true}
}

func (o *Object) key(key string) {
	if len(o.buf) > o.start {
		o.buf = append(o.buf, ',')
	}

	o.buf = append(o.buf, '"')
	o.buf = append(o.buf, key...)
	o.buf = append(o.buf, '"', ':')
}

// String adds a string field, omitted when val is empty. key must be plain ASCII needing no escape.
func (o *Object) String(key, val string) {
	if val == "" || !o.ok {
		return
	}

	o.key(key)
	o.buf, o.ok = AppendString(o.buf, val)
}

// Int adds an integer field, omitted when n is zero. key must be plain ASCII needing no escape.
func (o *Object) Int(key string, n int64) {
	if n == 0 || !o.ok {
		return
	}

	o.key(key)
	o.buf = AppendInt(o.buf, n)
}

// End closes the object. It reports false if any field could not be encoded.
func (o *Object) End() ([]byte, bool) {
	return append(o.buf, '}'), o.ok
}
