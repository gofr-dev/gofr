package types

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

// CorrelationIDKey is the key denoting correlation ID
const CorrelationIDKey contextKey = "correlationID"
const correlationIDLength = 32

// CorrelationID is a 16-byte array with at least one non-zero byte.
// Used for tracing and logging
// It will be a part of every incoming request and will be propagated to every outgoing request
type CorrelationID string

// GetCorrelationIDFromContext retrieves the correlation ID from a context.
func GetCorrelationIDFromContext(ctx context.Context) CorrelationID {
	return CorrelationID(fmt.Sprint(ctx.Value(CorrelationIDKey)))
}

// String returns the string representation of a CorrelationID.
func (c CorrelationID) String() string {
	return string(c)
}

// Validate checks if a CorrelationID is valid.
func (c CorrelationID) Validate() error {
	_, err := trace.TraceIDFromHex(string(c))
	return err
}

// SetInContext sets a CorrelationID in a context.
func (c CorrelationID) SetInContext(ctx context.Context) (context.Context, error) {
	if err := c.Validate(); err != nil {
		return ctx, err
	}

	return context.WithValue(ctx, CorrelationIDKey, c), nil
}

// GenerateCorrelationID generates a new CorrelationID based on the provided context.
func GenerateCorrelationID(ctx context.Context) CorrelationID {
	cID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()
	// if tracing is not enabled, otel sets the trace-ID to "00000000000000000000000000000000" (nil type of [16]byte)
	nullCorrelationID := fmt.Sprintf("%0*s", correlationIDLength, "")

	if cID == nullCorrelationID {
		id, _ := uuid.NewUUID()
		s := strings.Split(id.String(), "-")

		cID = strings.Join(s, "")
	}

	return CorrelationID(cID)
}
