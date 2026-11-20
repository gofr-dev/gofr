package gofr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

type ctxOnlyServerStream struct {
	grpc.ServerStream

	ctx context.Context
}

func (s ctxOnlyServerStream) Context() context.Context { return s.ctx }

// assertTraceKeepsField checks that a Context built by hand from ctx, as gofr-cli's generated gRPC
// wrappers build it, already holds a box: Trace then moves the span without writing c.Context,
// which other goroutines may be reading (issue #3823).
func assertTraceKeepsField(t *testing.T, got any) {
	t.Helper()

	box, ok := got.(*ctxBox)
	require.True(t, ok, "the handler must receive a box, got %T", got)

	c := &Context{Context: box}

	span := c.Trace("work")
	defer span.End()

	assert.Same(t, box, c.Context, "Trace must not replace the field")
	assert.Equal(t, span.SpanContext(), oteltrace.SpanFromContext(c).SpanContext(), "the new span is current")
}

func TestBoxUnaryInterceptor_HandlerGetsBox(t *testing.T) {
	otel.SetTracerProvider(trace.NewTracerProvider())

	var got context.Context

	_, err := boxUnaryInterceptor(t.Context(), nil, &grpc.UnaryServerInfo{},
		func(ctx context.Context, _ any) (any, error) {
			got = ctx
			return nil, nil
		})
	require.NoError(t, err)

	assertTraceKeepsField(t, got)
}

func TestBoxStreamInterceptor_HandlerGetsBox(t *testing.T) {
	otel.SetTracerProvider(trace.NewTracerProvider())

	var got context.Context

	err := boxStreamInterceptor(nil, ctxOnlyServerStream{ctx: t.Context()}, &grpc.StreamServerInfo{},
		func(_ any, ss grpc.ServerStream) error {
			got = ss.Context()
			return nil
		})
	require.NoError(t, err)

	assertTraceKeepsField(t, got)
}
