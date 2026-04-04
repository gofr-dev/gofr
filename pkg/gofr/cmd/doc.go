// Package cmd provides abstractions for building command-line applications
// within the GoFr framework.
//
// It mirrors the HTTP handler pattern by parsing CLI flags into a [Request]
// that supports Param, PathParam, and struct Bind operations. The [Responder]
// writes successful output to stdout and errors to stderr, giving CLI
// applications the same handler-based structure used by GoFr HTTP services.
package cmd
