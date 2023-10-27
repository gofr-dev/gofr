package middleware

import (
	"context"
	"net/http"
)

type (
	clientIP            string
	authenticatedUserID string
	authorizationHeader string
	b3TraceID           string
)

const (
	ClientIPKey            clientIP            = "clientIP"
	AuthenticatedUserIDKey authenticatedUserID = "authUserID"
	AuthorizationHeader    authorizationHeader = "authorization"
	B3TraceIDKey           b3TraceID           = "b3traceID"
)

// PropagateHeaders propagates all the required headers through the context
func PropagateHeaders(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trueClientIP := r.Header.Get("True-Client-IP")
		authUserID := r.Header.Get("X-Authenticated-UserId")
		authorizationHeader := r.Header.Get("Authorization")
		b3TraceID := r.Header.Get("X-B3-TraceID")

		ctx := context.WithValue(r.Context(), ClientIPKey, trueClientIP)

		if b3TraceID != "" {
			ctx = context.WithValue(ctx, B3TraceIDKey, b3TraceID)
		}

		if authUserID != "" {
			ctx = context.WithValue(ctx, AuthenticatedUserIDKey, authUserID)
		}

		if authorizationHeader != "" {
			ctx = context.WithValue(ctx, AuthorizationHeader, authorizationHeader)
		}

		*r = *r.Clone(ctx)

		inner.ServeHTTP(w, r)
	})
}
