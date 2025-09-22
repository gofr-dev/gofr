package grpc

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
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

// Mock implementations for testing
type mockLogger struct {
	infoCalls  []interface{}
	errorCalls []interface{}
	debugCalls []interface{}
}

func (m *mockLogger) Info(args ...any) {
	m.infoCalls = append(m.infoCalls, args)
}

func (m *mockLogger) Errorf(format string, args ...any) {
	m.errorCalls = append(m.errorCalls, []interface{}{format, args})
}

func (m *mockLogger) Debug(args ...any) {
	m.debugCalls = append(m.debugCalls, args)
}

type mockMetrics struct {
	histogramCalls []interface{}
}

func (m *mockMetrics) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	m.histogramCalls = append(m.histogramCalls, []interface{}{ctx, name, value, labels})
}

func TestNewgRPCLogger(t *testing.T) {
	logger := NewgRPCLogger()

	assert.Equal(t, gRPCLog{}, logger)
}

func TestGRPCLog_DocumentRPCLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := &mockLogger{}
	mockMetrics := &mockMetrics{}

	log := gRPCLog{}
	ctx := context.Background()
	start := time.Now()
	err := status.Error(codes.Internal, "test error")
	method := "test.method"
	name := "test_metric"

	log.DocumentRPCLog(ctx, mockLogger, mockMetrics, start, err, method, name)

	// Verify logger was called
	assert.Len(t, mockLogger.infoCalls, 1)
	assert.Len(t, mockMetrics.histogramCalls, 1)
}

func TestObservabilityInterceptor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := &mockLogger{}
	mockMetrics := &mockMetrics{}

	interceptor := ObservabilityInterceptor(mockLogger, mockMetrics)

	ctx := context.Background()
	req := "test request"
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "test response", nil
	}

	resp, err := interceptor(ctx, req, info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "test response", resp)
	assert.Len(t, mockLogger.infoCalls, 1)
	assert.Len(t, mockMetrics.histogramCalls, 1)
}

func TestObservabilityInterceptor_WithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := &mockLogger{}
	mockMetrics := &mockMetrics{}

	interceptor := ObservabilityInterceptor(mockLogger, mockMetrics)

	ctx := context.Background()
	req := "test request"
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.Internal, "test error")
	}

	resp, err := interceptor(ctx, req, info, handler)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Len(t, mockLogger.errorCalls, 1)
	assert.Len(t, mockMetrics.histogramCalls, 1)
}

func TestObservabilityInterceptor_HealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := &mockLogger{}
	mockMetrics := &mockMetrics{}

	interceptor := ObservabilityInterceptor(mockLogger, mockMetrics)

	ctx := context.Background()
	req := &grpc_health_v1.HealthCheckRequest{
		Service: "test-service",
	}
	info := &grpc.UnaryServerInfo{
		FullMethod: healthCheck,
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return &grpc_health_v1.HealthCheckResponse{}, nil
	}

	resp, err := interceptor(ctx, req, info, handler)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, mockLogger.infoCalls, 1)
	assert.Len(t, mockMetrics.histogramCalls, 1)
}

func TestStreamObservabilityInterceptor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := &mockLogger{}
	mockMetrics := &mockMetrics{}

	interceptor := StreamObservabilityInterceptor(mockLogger, mockMetrics)

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: false,
		IsServerStream: true,
	}

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return nil
	}

	err := interceptor(nil, &mockServerStream{}, info, handler)

	assert.NoError(t, err)
	assert.Len(t, mockLogger.infoCalls, 1)
	assert.Len(t, mockMetrics.histogramCalls, 1)
}

func TestStreamObservabilityInterceptor_ClientStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := &mockLogger{}
	mockMetrics := &mockMetrics{}

	interceptor := StreamObservabilityInterceptor(mockLogger, mockMetrics)

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: true,
		IsServerStream: false,
	}

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return nil
	}

	err := interceptor(nil, &mockServerStream{}, info, handler)

	assert.NoError(t, err)
	assert.Len(t, mockLogger.infoCalls, 1)
	assert.Len(t, mockMetrics.histogramCalls, 1)
}

func TestStreamObservabilityInterceptor_BidirectionalStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := &mockLogger{}
	mockMetrics := &mockMetrics{}

	interceptor := StreamObservabilityInterceptor(mockLogger, mockMetrics)

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: true,
		IsServerStream: true,
	}

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return nil
	}

	err := interceptor(nil, &mockServerStream{}, info, handler)

	assert.NoError(t, err)
	assert.Len(t, mockLogger.infoCalls, 1)
	assert.Len(t, mockMetrics.histogramCalls, 1)
}

func TestStreamObservabilityInterceptor_WithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := &mockLogger{}
	mockMetrics := &mockMetrics{}

	interceptor := StreamObservabilityInterceptor(mockLogger, mockMetrics)

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: false,
		IsServerStream: true,
	}

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return status.Error(codes.Internal, "stream error")
	}

	err := interceptor(nil, &mockServerStream{}, info, handler)

	assert.Error(t, err)
	assert.Len(t, mockLogger.infoCalls, 1)
	assert.Len(t, mockMetrics.histogramCalls, 1)
}

func TestInitializeSpanContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "no metadata",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name: "valid trace context",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"x-gofr-traceid", "12345678901234567890123456789012",
				"x-gofr-spanid", "1234567890123456",
			)),
			expected: true,
		},
		{
			name: "missing trace id",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"x-gofr-spanid", "1234567890123456",
			)),
			expected: false,
		},
		{
			name: "missing span id",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"x-gofr-traceid", "12345678901234567890123456789012",
			)),
			expected: false,
		},
		{
			name: "invalid trace id",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"x-gofr-traceid", "invalid",
				"x-gofr-spanid", "1234567890123456",
			)),
			expected: true, // Function creates span context even with invalid hex
		},
		{
			name: "invalid span id",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
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

func TestGetMetadataValue(t *testing.T) {
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

// Mock server stream for testing
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

func (m *mockServerStream) SendMsg(msg interface{}) error {
	return nil
}

func (m *mockServerStream) RecvMsg(msg interface{}) error {
	return nil
}

func TestWrappedServerStream_Context(t *testing.T) {
	ctx := context.WithValue(context.Background(), "test-key", "test-value")

	wrapped := &wrappedServerStream{
		ServerStream: &mockServerStream{},
		ctx:          ctx,
	}

	result := wrapped.Context()
	assert.Equal(t, ctx, result)
}
