package cmd

import (
	"fmt"
	"os"
)

// Responder writes a command's output to stdout and its error to stderr.
// It records whether an error was responded with, so the caller can set a
// non-zero process exit code (shells and CI rely on the exit status to detect
// a failed command).
type Responder struct {
	errored bool
}

// Respond writes data to stdout and err to stderr. If err is non-nil, it marks
// the responder as errored so the process can later exit with a non-zero status.
func (r *Responder) Respond(data any, err error) {
	if data != nil {
		fmt.Fprintln(os.Stdout, data)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		r.errored = true
	}
}

// Errored reports whether Respond was called with a non-nil error.
func (r *Responder) Errored() bool {
	return r.errored
}
