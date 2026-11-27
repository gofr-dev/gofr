// Package errhide holds the HIDE_INTERNAL_ERRORS switch and the rule that decides which error text
// is kept away from clients. HTTP responses, SSE/NDJSON frames, GraphQL and MCP apply Hides. gRPC
// reads only the switch: there an error is classified by its gRPC status, not by StatusCode().
package errhide

import (
	"sync/atomic"

	"gofr.dev/pkg/gofr/datasource"
)

// Message replaces the text of a hidden error.
const Message = "Internal Server Error"

//nolint:gochecknoglobals // set once in gofr.New and read on every error response; a single atomic load
var enabled atomic.Bool

// Enable turns hiding on or off. gofr.New calls it only to turn it on, never off, so the switch is
// process-wide: one app with HIDE_INTERNAL_ERRORS=true hides errors for every app in the process.
func Enable(on bool) {
	enabled.Store(on)
}

// Enabled reports whether hiding is on.
func Enabled() bool {
	return enabled.Load()
}

// Logger is what Reportf needs from a logger.
type Logger interface {
	Errorf(format string, args ...any)
}

//nolint:gochecknoglobals // process-wide like the switch; read only when an error is being replaced
var logger atomic.Pointer[Logger]

// SetLogger sets the logger Reportf writes to. gofr.New sets it when hiding is turned on; nil clears it.
func SetLogger(l Logger) {
	if l == nil {
		logger.Store(nil)
		return
	}

	logger.Store(&l)
}

// Reportf logs a line about an error whose text was replaced, for the places that have no logger of
// their own (a stream source, a middleware). It does nothing until SetLogger has been called.
func Reportf(format string, args ...any) {
	if l := logger.Load(); l != nil {
		(*l).Errorf(format, args...)
	}
}

// statusCoder is implemented by errors that declare an HTTP status code, such as GoFr's predefined
// errors and an application's own errors. Their text was written for clients.
type statusCoder interface {
	StatusCode() int
}

// Hides reports whether err's text must be replaced with Message. It does not consult Enabled.
//
// An error is hidden when it declares no status code or declares 0 (the HTTP responder sends that as 500),
// or is a datasource.ErrorDB, which declares 500 but usually carries the driver's own text. The status
// code is found by a direct type assertion, the same way the HTTP responder picks the status, so an error
// wrapped with fmt.Errorf("...: %w") is hidden even when the error inside it declares one.
func Hides(err error) bool {
	switch e := err.(type) { //nolint:errorlint // direct assertion on purpose: a %w-wrapped error must stay hidden
	case nil:
		return false
	case datasource.ErrorDB, *datasource.ErrorDB:
		return true
	case statusCoder:
		return e.StatusCode() == 0
	default:
		return true
	}
}

// Apply returns Message in place of err's text when hiding is on and Hides(err), else err's text.
func Apply(err error) string {
	if enabled.Load() && Hides(err) {
		return Message
	}

	return err.Error()
}
