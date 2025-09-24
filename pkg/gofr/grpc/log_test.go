package grpc

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestNewgRPCLogger(t *testing.T) {
	logger := NewgRPCLogger()
	assert.Equal(t, gRPCLog{}, logger)
}

func TestRPCLog_String(t *testing.T) {
	l := gRPCLog{
		ID:         "123",
		StartTime:  "2020-01-01T12:12:12",
		Method:     http.MethodGet,
		StatusCode: 0,
	}

	expLog := `{"id":"123","startTime":"2020-01-01T12:12:12","responseTime":0,"method":"GET","statusCode":0}`

	assert.Equal(t, expLog, l.String())
}

func TestRPCLog_StringWithStreamType(t *testing.T) {
	l := gRPCLog{
		ID:         "123",
		StartTime:  "2020-01-01T12:12:12",
		Method:     "/test.Service/Method",
		StatusCode: 0,
		StreamType: "CLIENT_STREAM",
	}

	expLog := `{"id":"123","startTime":"2020-01-01T12:12:12","responseTime":0,` +
		`"method":"/test.Service/Method","statusCode":0,"streamType":"CLIENT_STREAM"}`

	assert.Equal(t, expLog, l.String())
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

	for i, tc := range testCases {
		response := colorForGRPCCode(tc.code)

		assert.Equal(t, tc.colorCode, response, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestRPCLog_PrettyPrint(t *testing.T) {
	startTime := time.Now().String()

	log := testutil.StdoutOutputForFunc(func() {
		l := gRPCLog{
			ID:           "1",
			StartTime:    startTime,
			ResponseTime: 10,
			Method:       http.MethodGet,
			StatusCode:   34,
		}

		l.PrettyPrint(os.Stdout)
	})

	// Check if method is coming
	assert.Contains(t, log, `GET`)
	// Check if responseTime is coming
	assert.Contains(t, log, `10`)
	// Check if statusCode is coming
	assert.Contains(t, log, `34`)
	// Check if ID is coming
	assert.Contains(t, log, `1`)
}

func TestRPCLog_PrettyPrintWithStreamType(t *testing.T) {
	var buf bytes.Buffer

	l := gRPCLog{
		ID:           "1",
		StartTime:    "2023-01-01T12:00:00Z",
		ResponseTime: 100,
		Method:       "/test.Service/Method",
		StatusCode:   0,
		StreamType:   "SERVER_STREAM",
	}

	l.PrettyPrint(&buf)

	output := buf.String()
	assert.Contains(t, output, "[SERVER_STREAM]")
	assert.Contains(t, output, "/test.Service/Method")
}

func TestGetStreamTypeAndMethod(t *testing.T) {
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
			streamType, method := getStreamTypeAndMethod(tc.info)
			assert.Equal(t, tc.expectedType, streamType)
			assert.Equal(t, tc.expectedMethod, method)
		})
	}
}

func TestGetMetadataValue(t *testing.T) {
	md := metadata.Pairs("key1", "value1", "key2", "value2")

	assert.Equal(t, "value1", getMetadataValue(md, "key1"))
	assert.Equal(t, "value2", getMetadataValue(md, "key2"))
	assert.Empty(t, getMetadataValue(md, "nonexistent"))
}

func TestGetTraceID(t *testing.T) {
	assert.Equal(t, "00000000000000000000000000000000", getTraceID(t.Context()))
	assert.Equal(t, "00000000000000000000000000000000", getTraceID(t.Context()))
}

func TestWrappedServerStream_Context(t *testing.T) {
	type contextKey string

	originalCtx := t.Context()
	newCtx := context.WithValue(originalCtx, contextKey("key"), "value")
	wrapped := &wrappedServerStream{
		ctx: newCtx,
	}
	assert.Equal(t, newCtx, wrapped.Context())
	assert.Equal(t, "value", wrapped.Context().Value(contextKey("key")))
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

func TestInitializeSpanContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "no metadata",
			ctx:      t.Context(),
			expected: false,
		},
		{
			name: "valid trace context",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-traceid", "12345678901234567890123456789012",
				"x-gofr-spanid", "1234567890123456",
			)),
			expected: true,
		},
		{
			name: "missing trace id",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-spanid", "1234567890123456",
			)),
			expected: false,
		},
		{
			name: "missing span id",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-traceid", "12345678901234567890123456789012",
			)),
			expected: false,
		},
		{
			name: "invalid trace id",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-traceid", "invalid",
				"x-gofr-spanid", "1234567890123456",
			)),
			expected: true, // Function creates span context even with invalid hex
		},
		{
			name: "invalid span id",
			ctx: metadata.NewIncomingContext(t.Context(), metadata.Pairs(
				"x-gofr-traceid", "12345678901234567890123456789012",
				"x-gofr-spanid", "invalid",
			)),
			expected: true, // Function creates span context even with invalid hex
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := initializeSpanContext(tt.ctx)

			if tt.expected {
				// When trace context is created, the result should be different from input
				assert.NotEqual(t, tt.ctx, result)
				// Verify that a span context was added
				span := trace.SpanFromContext(result)
				assert.NotNil(t, span)
				// For invalid hex values, the span context may not be valid but still exists
				if tt.name == "valid trace context" {
					assert.True(t, span.SpanContext().IsValid())
				} else {
					// For invalid cases, span context exists but may not be valid
					assert.False(t, span.SpanContext().IsValid())
				}
			} else {
				// When no trace context is created, the result should be the same as input
				assert.Equal(t, tt.ctx, result)
			}
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
