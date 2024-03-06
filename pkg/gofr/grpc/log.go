package grpc

import (
	"context"
	"encoding/json"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Logger interface {
	Info(args ...interface{})
}

type RPCLog struct {
	ID           string `json:"id"`
	StartTime    string `json:"startTime"`
	ResponseTime int64  `json:"responseTime"`
	Method       string `json:"method"`
	StatusCode   int32  `json:"statusCode"`
}

func (l RPCLog) String() string {
	line, _ := json.Marshal(l)
	return string(line)
}

func LoggingInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, span := otel.GetTracerProvider().Tracer("gofr",
			trace.WithInstrumentationVersion("v0.1")).Start(ctx, info.FullMethod)
		start := time.Now()

		resp, err := handler(ctx, req)

		defer func() {
			l := RPCLog{
				ID:           trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
				StartTime:    start.Format("2006-01-02T15:04:05.999999999-07:00"),
				ResponseTime: time.Since(start).Microseconds(),
				Method:       info.FullMethod,
			}

			if err != nil {
				// Check if the error is a gRPC status error
				if statusErr, ok := status.FromError(err); ok {
					// You can access the gRPC status code here
					l.StatusCode = int32(statusErr.Code())
				}
			} else {
				// If there was no error, you can access the response status code here
				l.StatusCode = int32(codes.OK)
			}

			if logger != nil {
				logger.Info(l)
			}

			span.End()
		}()

		return resp, err
	}
}
