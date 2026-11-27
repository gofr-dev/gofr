//go:build !gofr_nogrpc

package gofr

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	"gofr.dev/pkg/gofr/testutil"
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

// ctxRecordingHealth is a health server that hands each Check's context to the test.
type ctxRecordingHealth struct {
	grpc_health_v1.UnimplementedHealthServer

	got chan context.Context
}

func (h *ctxRecordingHealth) Check(ctx context.Context, _ *grpc_health_v1.HealthCheckRequest) (
	*grpc_health_v1.HealthCheckResponse, error) {
	h.got <- ctx

	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

// TestGRPCServer_HandlerGetsBoxAfterLateInterceptor pins that the box interceptor runs after every
// user interceptor, including one added after the server was built, so a user interceptor that
// replaces the context cannot hand the handler a context without the box.
func TestGRPCServer_HandlerGetsBoxAfterLateInterceptor(t *testing.T) {
	otel.SetTracerProvider(trace.NewTracerProvider())
	t.Setenv("METRICS_PORT", "0")
	t.Setenv("HTTP_PORT", strconv.Itoa(testutil.GetFreePort(t)))

	type ctxKey struct{}

	replaceCtx := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(context.WithValue(ctx, ctxKey{}, "set"), req)
	}

	app := New()
	h := &ctxRecordingHealth{got: make(chan context.Context, 1)}
	app.RegisterService(&grpc_health_v1.Health_ServiceDesc, h)
	app.AddGRPCUnaryInterceptors(replaceCtx)

	client := grpcHealthClient(t, app) // builds the server

	app.AddGRPCUnaryInterceptors(replaceCtx) // added late, so it runs through runLateUnary

	_, err := client.Check(t.Context(), &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	got := <-h.got
	assert.Equal(t, "set", got.Value(ctxKey{}), "both user interceptors ran before the handler")
	assertTraceKeepsField(t, got)
}
