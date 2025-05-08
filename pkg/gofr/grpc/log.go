package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	statusCodeWidth           = 3
	responseTimeWidth         = 11
	nanosecondsPerMillisecond = 1e6
	debugMethod               = "/grpc.health.v1.Health/SetServingStatus"
	healthCheck               = "/grpc.health.v1.Health/Check"
)

type Logger interface {
	Info(args ...any)
	Errorf(string, ...any)
	Debug(...any)
}

type Metrics interface {
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}

type gRPCLog struct {
	ID           string `json:"id"`
	StartTime    string `json:"startTime"`
	ResponseTime int64  `json:"responseTime"`
	Method       string `json:"method"`
	StatusCode   int32  `json:"statusCode"`
}

//nolint:revive // We intend to keep it non-exported for ease in future changes without any breaking change.
func NewgRPCLogger() gRPCLog {
	return gRPCLog{}
}

func (l gRPCLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%s \u001B[38;5;%dm%-*d"+
		"\u001B[0m %*d\u001B[38;5;8mµs\u001B[0m %s %s\n",
		l.ID, colorForGRPCCode(l.StatusCode),
		statusCodeWidth, l.StatusCode,
		responseTimeWidth, l.ResponseTime,
		"GRPC", l.Method)
}

func colorForGRPCCode(s int32) int {
	const (
		blue = 34
		red  = 202
	)

	if s == 0 {
		return blue
	}

	return red
}

func (l gRPCLog) String() string {
	line, _ := json.Marshal(l)
	return string(line)
}

// StreamObservabilityInterceptor handles logging, metrics, and tracing for streaming RPCs.
func StreamObservabilityInterceptor(logger Logger, metrics Metrics) grpc.StreamServerInterceptor {
	tracer := otel.GetTracerProvider().Tracer("gofr-gRPC-stream", trace.WithInstrumentationVersion("v0.1"))

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		// Initialize tracing context from incoming metadata
		ctx := initializeSpanContext(ss.Context())

		ctx, span := tracer.Start(ctx, info.FullMethod)

		defer span.End()

		// Wrap the stream to propagate context with tracing
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Process the stream
		err := handler(srv, wrappedStream)

		grpcMethodName := info.FullMethod
		if info.IsClientStream && info.IsServerStream {
			grpcMethodName += " [BI-DIRECTION_STREAM]"
		} else if info.IsClientStream {
			grpcMethodName += " [CLIENT-STREAM]"
		} else if info.IsServerStream {
			grpcMethodName += " [SERVER-STREAM]"
		}

		// Log and record metrics
		logRPC(ctx, logger, metrics, start, err, grpcMethodName, "app_gRPC-Stream_stats")

		return err
	}
}

// wrappedServerStream propagates context with tracing for streaming RPCs.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func ObservabilityInterceptor(logger Logger, metrics Metrics) grpc.UnaryServerInterceptor {
	tracer := otel.GetTracerProvider().Tracer("gofr", trace.WithInstrumentationVersion("v0.1"))

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		ctx = initializeSpanContext(ctx)
		ctx, span := tracer.Start(ctx, info.FullMethod)

		resp, err := handler(ctx, req)
		if err != nil {
			logger.Errorf("error while handling gRPC request to method %q: %q", info.FullMethod, err)
		}

		if info.FullMethod == healthCheck {
			service, ok := req.(*grpc_health_v1.HealthCheckRequest)
			if ok {
				info.FullMethod = fmt.Sprintf("%s	Service: %q", healthCheck, service.Service)
			}
		}

		logRPC(ctx, logger, metrics, start, err, info.FullMethod, "app_gRPC-Server_stats")

		span.End()

		return resp, err
	}
}

func initializeSpanContext(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)

	traceIDHex := getMetadataValue(md, "x-gofr-traceid")
	spanIDHex := getMetadataValue(md, "x-gofr-spanid")

	if traceIDHex == "" || spanIDHex == "" {
		return ctx
	}

	traceID, _ := trace.TraceIDFromHex(traceIDHex)
	spanID, _ := trace.SpanIDFromHex(spanIDHex)

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)

	return ctx
}

func (gRPCLog) DocumentRPCLog(ctx context.Context, logger Logger, metrics Metrics, start time.Time, err error, method, name string) {
	logRPC(ctx, logger, metrics, start, err, method, name)
}

func logRPC(ctx context.Context, logger Logger, metrics Metrics, start time.Time, err error, method, name string) {
	duration := time.Since(start)

	logEntry := gRPCLog{
		ID:           trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
		StartTime:    start.Format("2006-01-02T15:04:05.999999999-07:00"),
		ResponseTime: time.Since(start).Microseconds(),
		Method:       method,
	}

	if err != nil {
		statusErr, _ := status.FromError(err)
		//nolint:gosec // gRPC codes are typically under the range.
		logEntry.StatusCode = int32(statusErr.Code())
	} else {
		logEntry.StatusCode = int32(codes.OK)
	}

	if logger != nil {
		switch {
		case method == debugMethod,
			strings.Contains(method, "/Send"),
			strings.Contains(method, "/Recv"),
			strings.Contains(method, "/SendAndClose"):
			logger.Debug(logEntry)
		default:
			logger.Info(logEntry)
		}
	}

	if metrics != nil {
		metrics.RecordHistogram(ctx, name,
			float64(duration.Milliseconds())+float64(duration.Nanoseconds()%nanosecondsPerMillisecond)/nanosecondsPerMillisecond,
			"method",
			method)
	}
}

// Helper function to safely extract a value from metadata.
func getMetadataValue(md metadata.MD, key string) string {
	if values, ok := md[key]; ok && len(values) > 0 {
		return values[0]
	}

	return ""
}
