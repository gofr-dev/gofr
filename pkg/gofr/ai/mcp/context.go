package mcp

import (
	"context"
	"net/http"
)

type headerContextKey struct{}

// WithHeaders returns a copy of ctx carrying the inbound request headers so a downstream tool
// dispatcher can rebuild an authenticated request for the caller.
func WithHeaders(ctx context.Context, h http.Header) context.Context {
	return context.WithValue(ctx, headerContextKey{}, h)
}

// HeadersFromContext returns the headers stored by WithHeaders, or nil if none are present.
func HeadersFromContext(ctx context.Context) http.Header {
	h, _ := ctx.Value(headerContextKey{}).(http.Header)

	return h
}
