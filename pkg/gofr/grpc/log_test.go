package grpc

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func Test_NewgRPCLogger(t *testing.T) {
	testCases := []struct {
		name     string
		expected gRPCLog
	}{
		{
			name:     "empty",
			expected: gRPCLog{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger := NewgRPCLogger()
			assert.Equal(t, tc.expected, logger)
		})
	}
}

func Test_RPCLog_String(t *testing.T) {
	testCases := []struct {
		name     string
		entry    gRPCLog
		expected string
	}{
		{
			name: "without_stream",
			entry: gRPCLog{
				ID:         "123",
				StartTime:  "2020-01-01T12:12:12",
				Method:     http.MethodGet,
				StatusCode: 0,
			},
			expected: `{"id":"123","startTime":"2020-01-01T12:12:12","responseTime":0,"method":"GET","statusCode":0}`,
		},
		{
			name: "with_stream",
			entry: gRPCLog{
				ID:         "123",
				StartTime:  "2020-01-01T12:12:12",
				Method:     "/test.Service/Method",
				StatusCode: 0,
				StreamType: "CLIENT_STREAM",
			},
			expected: `{"id":"123","startTime":"2020-01-01T12:12:12","responseTime":0,"method":"/test.Service/Method","statusCode":0,"streamType":"CLIENT_STREAM"}`, //nolint:lll // keep JSON on one line to assert exact string output
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, tc.entry.String())
		})
	}
}

func Test_colorForGRPCCode(t *testing.T) {
	testCases := []struct {
		desc      string
		code      int32
		colorCode int
	}{
		{"code 0", 0, 34},
		{"negative code", -1, 202},
		{"positive code", 1, 202},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			response := colorForGRPCCode(tc.code)
			assert.Equal(t, tc.colorCode, response)
		})
	}
}

func Test_RPCLog_PrettyPrint(t *testing.T) {
	testCases := []struct {
		name string
	}{
		{name: "stdout"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			startTime := time.Now().String()

			output := testutil.StdoutOutputForFunc(func() {
				logEntry := gRPCLog{
					ID:           "1",
					StartTime:    startTime,
					ResponseTime: 10,
					Method:       http.MethodGet,
					StatusCode:   34,
				}

				logEntry.PrettyPrint(os.Stdout)
			})

			assert.Contains(t, output, "GET")
			assert.Contains(t, output, "10")
			assert.Contains(t, output, "34")
			assert.Contains(t, output, "1")
		})
	}
}

func Test_RPCLog_PrettyPrintWithStreamType(t *testing.T) {
	testCases := []struct {
		name      string
		streamTag string
	}{
		{
			name:      "buffer",
			streamTag: "[SERVER_STREAM]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			logEntry := gRPCLog{
				ID:           "1",
				StartTime:    "2023-01-01T12:00:00Z",
				ResponseTime: 100,
				Method:       "/test.Service/Method",
				StatusCode:   0,
				StreamType:   "SERVER_STREAM",
			}

			logEntry.PrettyPrint(&buf)

			output := buf.String()
			assert.Contains(t, output, tc.streamTag)
			assert.Contains(t, output, "/test.Service/Method")
		})
	}
}

func Test_GetStreamTypeAndMethod(t *testing.T) {
	testCases := []struct {
		desc           string
		info           *grpc.StreamServerInfo
		expectedType   string
		expectedMethod string
	}{
		{
			desc: "bidirectional stream",
			info: &grpc.StreamServerInfo{
				FullMethod:     "/test.Service/Method",
				IsClientStream: true,
				IsServerStream: true,
			},
			expectedType:   "BIDIRECTIONAL",
			expectedMethod: "/test.Service/Method [BI-DIRECTION_STREAM]",
		},
		{
			desc: "client stream",
			info: &grpc.StreamServerInfo{
				FullMethod:     "/test.Service/Method",
				IsClientStream: true,
				IsServerStream: false,
			},
			expectedType:   "CLIENT_STREAM",
			expectedMethod: "/test.Service/Method [CLIENT-STREAM]",
		},
		{
			desc: "server stream",
			info: &grpc.StreamServerInfo{
				FullMethod:     "/test.Service/Method",
				IsClientStream: false,
				IsServerStream: true,
			},
			expectedType:   "SERVER_STREAM",
			expectedMethod: "/test.Service/Method [SERVER-STREAM]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			streamType, method := getStreamTypeAndMethod(tc.info)
			assert.Equal(t, tc.expectedType, streamType)
			assert.Equal(t, tc.expectedMethod, method)
		})
	}
}

func Test_GetMetadataValue(t *testing.T) {
	md := metadata.Pairs("key1", "value1", "key2", "value2")

	testCases := []struct {
		name     string
		key      string
		expected string
	}{
		{name: "present_first", key: "key1", expected: "value1"},
		{name: "present_second", key: "key2", expected: "value2"},
		{name: "absent", key: "nonexistent", expected: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, getMetadataValue(md, tc.key))
		})
	}
}

func Test_GetTraceID(t *testing.T) {
	testCases := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{name: "nil_ctx", ctx: nil, expected: ""},
		{name: "background", ctx: context.Background(), expected: "00000000000000000000000000000000"},
		{name: "todo", ctx: context.TODO(), expected: "00000000000000000000000000000000"},
		{name: "with_metadata_no_span", ctx: metadata.NewIncomingContext(context.Background(),
			metadata.Pairs("key", "value")), expected: "00000000000000000000000000000000"},
		{name: "with_span", ctx: contextWithSpan(), expected: "1234567890abcdef1234567890abcdef"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := getTraceID(tc.ctx)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_InitializeSpanContext_WithHeaders(t *testing.T) {
	testCases := []struct {
		name    string
		traceID string
		spanID  string
	}{
		{
			name:    "valid_headers",
			traceID: "1234567890abcdef1234567890abcdef",
			spanID:  "1234567890abcdef",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := newIncomingCtx(t, tc.traceID, tc.spanID)

			enriched := initializeSpanContext(ctx)

			spanContext := trace.SpanFromContext(enriched).SpanContext()
			assert.True(t, spanContext.IsRemote())
			assert.Equal(t, tc.traceID, spanContext.TraceID().String())
			assert.Equal(t, tc.spanID, spanContext.SpanID().String())
		})
	}
}

func Test_InitializeSpanContext_WithoutHeaders_NoHeaders(t *testing.T) {
	baseCtx := context.Background()
	cancelCtx, cancel := context.WithCancel(baseCtx)
	t.Cleanup(cancel)

	enriched := initializeSpanContext(cancelCtx)

	assert.Equal(t, cancelCtx, enriched)
	assert.False(t, trace.SpanFromContext(enriched).SpanContext().IsValid())
}

func Test_InitializeSpanContextWithoutHeaders_MissingParts(t *testing.T) {
	testCases := []struct {
		name  string
		pairs []string
	}{
		{name: "missing_traceid", pairs: []string{"x-gofr-spanid", "1234567890abcdef"}},
		{name: "missing_spanid", pairs: []string{"x-gofr-traceid", "1234567890abcdef1234567890abcdef"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			baseCtx := context.Background()
			cancelCtx, cancel := context.WithCancel(baseCtx)
			t.Cleanup(cancel)

			ctx := metadata.NewIncomingContext(cancelCtx, metadata.Pairs(tc.pairs...))
			enriched := initializeSpanContext(ctx)

			assert.Equal(t, ctx, enriched)
			assert.False(t, trace.SpanFromContext(enriched).SpanContext().IsValid())
		})
	}
}

func Test_LogEntry_Routing(t *testing.T) {
	testCases := []struct {
		name          string
		method        string
		expectedDebug int
		expectedInfo  int
	}{
		{
			name:          "streaming",
			method:        "/foo.Service/Send",
			expectedDebug: 1,
			expectedInfo:  0,
		},
		{
			name:          "standard",
			method:        "/foo.Service/Execute",
			expectedDebug: 0,
			expectedInfo:  1,
		},
		{
			name:          "debug_method_exact",
			method:        debugMethod,
			expectedDebug: 1,
			expectedInfo:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger := newTestLogger()
			entry := &gRPCLog{Method: "ignored"}

			logGRPCEntry(logger, entry, tc.method)

			assert.Equal(t, tc.expectedDebug, logger.count("debug"))
			assert.Equal(t, tc.expectedInfo, logger.count("info"))
		})
	}
}

func Test_LogEntry_NilLogger(t *testing.T) {
	testCases := []struct {
		name   string
		method string
	}{
		{name: "standard", method: "/foo.Service/Execute"},
		{name: "streaming", method: "/foo.Service/Send"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			entry := &gRPCLog{Method: "ignored"}

			assert.NotPanics(t, func() {
				logGRPCEntry(nil, entry, tc.method)
			})
		})
	}
}

func Test_RecordGRPCMetrics(t *testing.T) {
	testCases := []struct {
		name           string
		duration       time.Duration
		streamType     string
		expectedLabels []string
	}{
		{
			name:           "with_stream_labels",
			duration:       1500 * time.Microsecond,
			streamType:     "CLIENT_STREAM",
			expectedLabels: []string{"method", "method", "stream_type", "CLIENT_STREAM"},
		},
		{
			name:           "no_stream_label",
			duration:       0,
			streamType:     "",
			expectedLabels: []string{"method", "method"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			cancelCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			sampleMetrics := newSampleMetrics()
			metrics := Metrics(sampleMetrics)

			recordGRPCMetrics(cancelCtx, metrics, "metric", tc.duration, "method", tc.streamType)

			assert.Len(t, sampleMetrics.calls, 1)
			assert.Equal(t, "metric", sampleMetrics.calls[0].name)
			assert.Equal(t, cancelCtx, sampleMetrics.calls[0].ctx)
			assert.Equal(t, tc.expectedLabels, sampleMetrics.calls[0].labels)
			assert.InDelta(t, float64(tc.duration.Milliseconds())+float64(tc.duration.Nanoseconds()%1000000)/1000000,
				sampleMetrics.calls[0].value, 0.0001)
		})
	}
}

func Test_RecordGRPCMetrics_Nil(t *testing.T) {
	ctx := context.Background()
	cancelCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	assert.NotPanics(t, func() {
		recordGRPCMetrics(cancelCtx, nil, "metric", time.Second, "method", "")
	})
}

func Test_LogRPCStatuses(t *testing.T) {
	testCases := []struct {
		name         string
		err          error
		expectedCode codes.Code
	}{
		{
			name:         "success",
			err:          nil,
			expectedCode: codes.OK,
		},
		{
			name:         "not_found",
			err:          status.Error(codes.NotFound, "missing"),
			expectedCode: codes.NotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger := newTestLogger()
			sampleMetrics := newSampleMetrics()
			ctx := contextWithSpan()
			cancelCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			logRPC(cancelCtx, logger, sampleMetrics, time.Unix(0, 0), tc.err, "method",
				"metric")

			assert.Equal(t, 1, logger.count("info"))
			assert.Equal(t, int(tc.expectedCode), int(logger.firstLogEntry("info").StatusCode))
			assert.Len(t, sampleMetrics.calls, 1)
			assert.Equal(t, "metric", sampleMetrics.calls[0].name)
			assert.Equal(t, []string{"method", "method"}, sampleMetrics.calls[0].labels)
		})
	}
}

func Test_LogStreamRPCStatuses(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		stream         string
		expectedCode   codes.Code
		expectedLabels []string
	}{
		{
			name:           "success",
			err:            nil,
			stream:         "SERVER_STREAM",
			expectedCode:   codes.OK,
			expectedLabels: []string{"method", "method", "stream_type", "SERVER_STREAM"},
		},
		{
			name:           "permission_denied",
			err:            status.Error(codes.PermissionDenied, "denied"),
			stream:         "CLIENT_STREAM",
			expectedCode:   codes.PermissionDenied,
			expectedLabels: []string{"method", "method", "stream_type", "CLIENT_STREAM"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger := newTestLogger()
			sampleMetrics := newSampleMetrics()
			ctx := contextWithSpan()
			cancelCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			logStreamRPC(cancelCtx, logger, sampleMetrics, time.Unix(0, 0), tc.err, "method",
				tc.stream, "metric")

			assert.Equal(t, 1, logger.count("info"))
			assert.Equal(t, int(tc.expectedCode), int(logger.firstLogEntry("info").StatusCode))
			assert.Len(t, sampleMetrics.calls, 1)
			assert.Equal(t, tc.expectedLabels, sampleMetrics.calls[0].labels)
		})
	}
}

func Test_DocumentRPCLogDelegation(t *testing.T) {
	testCases := []struct {
		name string
	}{
		{name: "delegates"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger := newTestLogger()
			sampleMetrics := newSampleMetrics()
			ctx := contextWithSpan()
			cancelCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			NewgRPCLogger().DocumentRPCLog(cancelCtx, logger, sampleMetrics, time.Unix(0, 0), nil,
				"method", "metric")

			assert.Equal(t, 1, logger.count("info"))
			assert.Len(t, sampleMetrics.calls, 1)
		})
	}
}

func Test_UnaryInterceptor_Success(t *testing.T) {
	logger := newTestLogger()
	sampleMetrics := newSampleMetrics()
	cancelCtx := newIncomingCtx(t, "1234567890abcdef1234567890abcdef", "1234567890abcdef")

	interceptor := ObservabilityInterceptor(logger, sampleMetrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/service/Method"}
	handler := func(_ context.Context, _ any) (any, error) {
		return "response", nil
	}

	resp, err := interceptor(cancelCtx, "request", info, handler)

	require.NoError(t, err)
	assert.Equal(t, "response", resp)
	assert.Equal(t, 0, logger.count("error"))
	assert.Equal(t, 1, logger.count("info"))
	assert.Equal(t, int32(codes.OK), logger.firstLogEntry("info").StatusCode)
	assert.Len(t, sampleMetrics.calls, 1)
}

func Test_UnaryInterceptor_Error(t *testing.T) {
	logger := newTestLogger()
	sampleMetrics := newSampleMetrics()
	cancelCtx := newIncomingCtx(t, "1234567890abcdef1234567890abcdef", "1234567890abcdef")

	interceptor := ObservabilityInterceptor(logger, sampleMetrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/service/Method"}
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, status.Error(codes.NotFound, "missing")
	}

	resp, err := interceptor(cancelCtx, "request", info, handler)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 1, logger.count("error"))
	assert.Equal(t, 1, logger.count("info"))
	assert.Equal(t, int32(codes.NotFound), logger.firstLogEntry("info").StatusCode)
	assert.Len(t, sampleMetrics.calls, 1)
}

func Test_UnaryInterceptor_HealthCheck(t *testing.T) {
	logger := newTestLogger()
	sampleMetrics := newSampleMetrics()
	cancelCtx := newIncomingCtx(t, "1234567890abcdef1234567890abcdef", "1234567890abcdef")

	interceptor := ObservabilityInterceptor(logger, sampleMetrics)
	info := &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}
	request := &grpc_health_v1.HealthCheckRequest{Service: "svc"}

	handler := func(_ context.Context, _ any) (any, error) {
		return nil, nil
	}

	resp, err := interceptor(cancelCtx, request, info, handler)

	require.NoError(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 0, logger.count("error"))
	assert.Equal(t, 1, logger.count("info"))
	assert.Equal(t, int32(codes.OK), logger.firstLogEntry("info").StatusCode)
	assert.Len(t, sampleMetrics.calls, 1)

	entry := logger.firstLogEntry("info")
	assert.Contains(t, entry.Method, "grpc.health.v1.Health/Check")
	assert.Contains(t, entry.Method, "Service: \"svc\"")
}

func Test_StreamInterceptor_Success(t *testing.T) {
	logger := newTestLogger()
	sampleMetrics := newSampleMetrics()
	cancelCtx := newIncomingCtx(t, "1234567890abcdef1234567890abcdef", "1234567890abcdef")

	interceptor := StreamObservabilityInterceptor(logger, sampleMetrics)
	info := &grpc.StreamServerInfo{FullMethod: "/service/Method", IsServerStream: true}
	stream := &sampleServerStream{ctx: cancelCtx}
	handler := func(any, grpc.ServerStream) error {
		return nil
	}

	err := interceptor(nil, stream, info, handler)

	require.NoError(t, err)
	assert.Equal(t, 1, logger.count("info"))
	assert.Len(t, sampleMetrics.calls, 1)
	assert.Equal(t, "SERVER_STREAM", logger.firstLogEntry("info").StreamType)
}

func Test_StreamInterceptor_Error(t *testing.T) {
	logger := newTestLogger()
	sampleMetrics := newSampleMetrics()
	cancelCtx := newIncomingCtx(t, "1234567890abcdef1234567890abcdef", "1234567890abcdef")

	interceptor := StreamObservabilityInterceptor(logger, sampleMetrics)
	info := &grpc.StreamServerInfo{FullMethod: "/service/Method", IsClientStream: true}
	stream := &sampleServerStream{ctx: cancelCtx}
	handler := func(any, grpc.ServerStream) error {
		return status.Error(codes.Aborted, "boom")
	}

	err := interceptor(nil, stream, info, handler)

	require.Error(t, err)
	assert.Equal(t, 1, logger.count("info"))
	assert.Equal(t, int32(codes.Aborted), logger.firstLogEntry("info").StatusCode)
	assert.Len(t, sampleMetrics.calls, 1)
}

func Test_WrappedServerStream_Context(t *testing.T) {
	t.Parallel()

	type contextKey string

	testCases := []struct {
		name  string
		setup func() (context.Context, contextKey)
	}{
		{
			name: "propagates",
			setup: func() (context.Context, contextKey) {
				key := contextKey("key")
				ctx := context.WithValue(context.Background(), key, "value")
				return ctx, key
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, key := tc.setup()
			wrapped := &wrappedServerStream{ctx: ctx}

			assert.Equal(t, ctx, wrapped.Context())
			assert.Equal(t, "value", wrapped.Context().Value(key))
		})
	}
}

func Test_GetStreamTypeNoStreaming(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		info *grpc.StreamServerInfo
	}{
		{
			name: "unary",
			info: &grpc.StreamServerInfo{FullMethod: "/service/Method"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			streamType, method := getStreamTypeAndMethod(tc.info)
			assert.Empty(t, streamType)
			assert.Equal(t, "/service/Method", method)
		})
	}
}

type loggedCall struct {
	level string
	args  []any
}

type testLogger struct {
	base  logging.Logger
	calls []loggedCall
}

func newTestLogger() *testLogger {
	logger := logging.NewFileLogger("")
	logger.ChangeLevel(logging.DEBUG)

	return &testLogger{base: logger}
}

func (l *testLogger) Info(args ...any) {
	l.calls = append(l.calls, loggedCall{level: "info", args: args})
	l.base.Info(args...)
}

func (l *testLogger) Errorf(format string, args ...any) {
	l.calls = append(l.calls, loggedCall{level: "error", args: []any{fmt.Sprintf(format, args...)}})
	l.base.Errorf(format, args...)
}

func (l *testLogger) Debug(args ...any) {
	l.calls = append(l.calls, loggedCall{level: "debug", args: args})
	l.base.Debug(args...)
}

func (l *testLogger) Fatalf(format string, args ...any) {
	l.calls = append(l.calls, loggedCall{level: "fatal", args: []any{fmt.Sprintf(format, args...)}})
}

func (l *testLogger) count(level string) int {
	n := 0

	for _, c := range l.calls {
		if c.level == level {
			n++
		}
	}

	return n
}

//nolint:unparam // level is kept for future-proofing; currently tests use "info"
func (l *testLogger) firstLogEntry(level string) *gRPCLog {
	for _, c := range l.calls {
		if c.level == level {
			if len(c.args) > 0 {
				if e, ok := c.args[0].(*gRPCLog); ok {
					return e
				}
			}

			break
		}
	}

	return nil
}

type metricsCall struct {
	ctx    context.Context
	name   string
	value  float64
	labels []string
}

type sampleMetrics struct {
	calls []metricsCall
}

func newSampleMetrics() *sampleMetrics {
	return &sampleMetrics{}
}

func (m *sampleMetrics) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	copied := append([]string(nil), labels...)
	m.calls = append(m.calls, metricsCall{ctx: ctx, name: name, value: value, labels: copied})
}

type sampleServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *sampleServerStream) Context() context.Context {
	return f.ctx
}

// newIncomingCtx returns an Incoming gRPC context with gofr trace/span IDs and registers cancel via t.Cleanup.
func newIncomingCtx(t *testing.T, traceID, spanID string) context.Context {
	t.Helper()

	md := metadata.Pairs("x-gofr-traceid", traceID, "x-gofr-spanid", spanID)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	return ctx
}

func contextWithSpan() context.Context {
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90,
			0xab, 0xcd, 0xef},
		SpanID:     trace.SpanID{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		TraceFlags: trace.FlagsSampled,
	})

	return trace.ContextWithSpanContext(context.Background(), spanContext)
}

// Helper function to create mock logger and metrics for testing.
func createMocks(t *testing.T) (*container.MockLogger, *container.MockMetrics, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockLogger := container.NewMockLogger(ctrl)
	mockMetrics := container.NewMockMetrics(ctrl)

	return mockLogger, mockMetrics, ctrl
}

func TestGRPCLog_DocumentRPCLog(t *testing.T) {
	mockLogger, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	log := gRPCLog{}
	ctx := t.Context()
	start := time.Now()
	err := status.Error(codes.Internal, "test error")
	method := "test.method"
	name := "test_metric"

	// Set up expectations
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	log.DocumentRPCLog(ctx, mockLogger, mockMetrics, start, err, method, name)
}

func TestObservabilityInterceptor(t *testing.T) {
	mockLogger, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	interceptor := ObservabilityInterceptor(mockLogger, mockMetrics)

	ctx := t.Context()
	req := "test request"
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	handler := func(_ context.Context, _ any) (any, error) {
		return "test response", nil
	}

	// Set up expectations
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	resp, err := interceptor(ctx, req, info, handler)

	require.NoError(t, err)
	assert.Equal(t, "test response", resp)
}

func TestObservabilityInterceptor_WithError(t *testing.T) {
	mockLogger, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	interceptor := ObservabilityInterceptor(mockLogger, mockMetrics)

	ctx := t.Context()
	req := "test request"
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	handler := func(_ context.Context, _ any) (any, error) {
		return nil, status.Error(codes.Internal, "test error")
	}

	// Set up expectations - the function logs errors with Errorf and then with Info
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	resp, err := interceptor(ctx, req, info, handler)

	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestObservabilityInterceptor_HealthCheck(t *testing.T) {
	mockLogger, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	interceptor := ObservabilityInterceptor(mockLogger, mockMetrics)

	ctx := t.Context()
	req := &grpc_health_v1.HealthCheckRequest{
		Service: "test-service",
	}
	info := &grpc.UnaryServerInfo{
		FullMethod: healthCheck,
	}

	handler := func(_ context.Context, _ any) (any, error) {
		return &grpc_health_v1.HealthCheckResponse{}, nil
	}

	// Set up expectations
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	resp, err := interceptor(ctx, req, info, handler)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestStreamObservabilityInterceptor(t *testing.T) {
	mockLogger, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	interceptor := StreamObservabilityInterceptor(mockLogger, mockMetrics)

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: false,
		IsServerStream: true,
	}

	handler := func(_ any, _ grpc.ServerStream) error {
		return nil
	}

	// Set up expectations
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	err := interceptor(nil, &mockServerStream{}, info, handler)

	require.NoError(t, err)
}

func TestStreamObservabilityInterceptor_ClientStream(t *testing.T) {
	mockLogger, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	interceptor := StreamObservabilityInterceptor(mockLogger, mockMetrics)

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: true,
		IsServerStream: false,
	}

	handler := func(_ any, _ grpc.ServerStream) error {
		return nil
	}

	// Set up expectations
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	err := interceptor(nil, &mockServerStream{}, info, handler)

	require.NoError(t, err)
}

func TestStreamObservabilityInterceptor_BidirectionalStream(t *testing.T) {
	mockLogger, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	interceptor := StreamObservabilityInterceptor(mockLogger, mockMetrics)

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: true,
		IsServerStream: true,
	}

	handler := func(_ any, _ grpc.ServerStream) error {
		return nil
	}

	// Set up expectations
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	err := interceptor(nil, &mockServerStream{}, info, handler)

	require.NoError(t, err)
}

func TestStreamObservabilityInterceptor_WithError(t *testing.T) {
	mockLogger, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	interceptor := StreamObservabilityInterceptor(mockLogger, mockMetrics)

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: false,
		IsServerStream: true,
	}

	handler := func(_ any, _ grpc.ServerStream) error {
		return status.Error(codes.Internal, "stream error")
	}

	// Set up expectations
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	err := interceptor(nil, &mockServerStream{}, info, handler)

	require.Error(t, err)
}

func TestInitializeSpanContext_NoSpanCreated(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{
			name: "no metadata",
			ctx:  t.Context(),
		},
		{
			name: "missing trace id",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-spanid", "1234567890123456",
			)),
		},
		{
			name: "missing span id",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-traceid", "12345678901234567890123456789012",
			)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := initializeSpanContext(tt.ctx)
			// When no trace context is created, the result should be the same as input
			assert.Equal(t, tt.ctx, result)
		})
	}
}

func TestInitializeSpanContext_SpanCreated(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectValid bool
	}{
		{
			name: "valid trace context",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-traceid", "12345678901234567890123456789012",
				"x-gofr-spanid", "1234567890123456",
			)),
			expectValid: true,
		},
		{
			name: "invalid trace id",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-traceid", "invalid",
				"x-gofr-spanid", "1234567890123456",
			)),
			expectValid: false, // Function creates span context even with invalid hex
		},
		{
			name: "invalid span id",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-traceid", "12345678901234567890123456789012",
				"x-gofr-spanid", "invalid",
			)),
			expectValid: false, // Function creates span context even with invalid hex
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := initializeSpanContext(tt.ctx)
			// When trace context is created, the result should be different from input
			assert.NotEqual(t, tt.ctx, result)
			// Verify that a span context was added
			span := trace.SpanFromContext(result)
			assert.NotNil(t, span)
			// Check if span context is valid based on test expectation
			assert.Equal(t, tt.expectValid, span.SpanContext().IsValid())
		})
	}
}

func TestGetMetadataValue_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		md       metadata.MD
		key      string
		expected string
	}{
		{
			name:     "key exists",
			md:       metadata.Pairs("test-key", "test-value"),
			key:      "test-key",
			expected: "test-value",
		},
		{
			name:     "key does not exist",
			md:       metadata.Pairs("other-key", "other-value"),
			key:      "test-key",
			expected: "",
		},
		{
			name:     "empty metadata",
			md:       metadata.MD{},
			key:      "test-key",
			expected: "",
		},
		{
			name:     "multiple values",
			md:       metadata.Pairs("test-key", "value1", "test-key", "value2"),
			key:      "test-key",
			expected: "value1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMetadataValue(tt.md, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock server stream for testing.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	if m.ctx == nil {
		return context.Background()
	}

	return m.ctx
}

func (*mockServerStream) SendMsg(_ any) error {
	return nil
}

func (*mockServerStream) RecvMsg(_ any) error {
	return nil
}

func TestWrappedServerStream_Context_Comprehensive(t *testing.T) {
	type testKey string

	ctx := context.WithValue(t.Context(), testKey("test-key"), "test-value")

	wrapped := &wrappedServerStream{
		ServerStream: &mockServerStream{},
		ctx:          ctx,
	}

	result := wrapped.Context()
	assert.Equal(t, ctx, result)
}

// Additional tests to reach 100% coverage.
func TestGetTraceID_WithSpanContext(t *testing.T) {
	// Test with context without span - this returns the default trace ID
	ctx := t.Context()
	traceID := getTraceID(ctx)
	assert.Equal(t, "00000000000000000000000000000000", traceID)
}

func TestGetTraceID_WithValidSpan(t *testing.T) {
	// Create a context with a valid span
	ctx := t.Context()
	span := trace.SpanFromContext(ctx)

	// Test with valid span context
	traceID := getTraceID(ctx)
	assert.NotEmpty(t, traceID)
	assert.Equal(t, span.SpanContext().TraceID().String(), traceID)
}

func TestGetTraceID_WithNilSpan(t *testing.T) {
	// Create a custom context that will make trace.SpanFromContext return nil
	// We'll use a context with a custom value that doesn't have a span
	type customKey string

	ctx := context.WithValue(t.Context(), customKey("custom-key"), "custom-value")

	// Test with context that has nil span
	traceID := getTraceID(ctx)
	assert.Equal(t, "00000000000000000000000000000000", traceID)
}

func TestLogGRPCEntry(t *testing.T) {
	mockLogger, _, ctrl := createMocks(t)
	defer ctrl.Finish()

	// Test logGRPCEntry function
	log := &gRPCLog{
		ID:           "test-id",
		StartTime:    "2023-01-01T12:00:00Z",
		ResponseTime: 100,
		Method:       "/test.Service/Method",
		StatusCode:   0,
	}

	// Set up expectations
	mockLogger.EXPECT().Info(gomock.Any()).Times(1)

	logGRPCEntry(mockLogger, log, "/test.Service/Method")
}

func TestLogGRPCEntry_WithDebugMethod(t *testing.T) {
	mockLogger, _, ctrl := createMocks(t)
	defer ctrl.Finish()

	// Test logGRPCEntry function with debug method
	log := &gRPCLog{
		ID:           "test-id",
		StartTime:    "2023-01-01T12:00:00Z",
		ResponseTime: 100,
		Method:       debugMethod,
		StatusCode:   0,
	}

	// Set up expectations
	mockLogger.EXPECT().Debug(gomock.Any()).Times(1)

	logGRPCEntry(mockLogger, log, debugMethod)
}

func TestLogGRPCEntry_WithSendMethod(t *testing.T) {
	mockLogger, _, ctrl := createMocks(t)
	defer ctrl.Finish()

	// Test logGRPCEntry function with Send method
	log := &gRPCLog{
		ID:           "test-id",
		StartTime:    "2023-01-01T12:00:00Z",
		ResponseTime: 100,
		Method:       "/test.Service/Send",
		StatusCode:   0,
	}

	// Set up expectations
	mockLogger.EXPECT().Debug(gomock.Any()).Times(1)

	logGRPCEntry(mockLogger, log, "/test.Service/Send")
}

func TestLogGRPCEntry_WithNilLogger(_ *testing.T) {
	// Test logGRPCEntry function with nil logger
	log := &gRPCLog{
		ID:           "test-id",
		StartTime:    "2023-01-01T12:00:00Z",
		ResponseTime: 100,
		Method:       "/test.Service/Method",
		StatusCode:   0,
	}

	// This should not panic
	logGRPCEntry(nil, log, "/test.Service/Method")
}

func TestRecordGRPCMetrics(t *testing.T) {
	_, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	ctx := t.Context()

	// Set up expectations
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	// Test recordGRPCMetrics function
	recordGRPCMetrics(ctx, mockMetrics, "test_metric", 100*time.Millisecond, "/test.Service/Method", "")
}

func TestRecordGRPCMetrics_WithStreamType(t *testing.T) {
	_, mockMetrics, ctrl := createMocks(t)
	defer ctrl.Finish()

	ctx := t.Context()

	// Set up expectations
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	// Test recordGRPCMetrics function with stream type
	recordGRPCMetrics(ctx, mockMetrics, "test_metric", 100*time.Millisecond, "/test.Service/Method", "SERVER_STREAM")
}

func TestRecordGRPCMetrics_WithNilMetrics(t *testing.T) {
	ctx := t.Context()

	// Test recordGRPCMetrics function with nil metrics
	// This should not panic
	recordGRPCMetrics(ctx, nil, "test_metric", 100*time.Millisecond, "/test.Service/Method", "")
}
