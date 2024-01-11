package grpc

import (
	"context"
	"encoding/json"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/logger"
)

type RPCLog struct {
	ID           string `json:"id"`
	StartTime    string `json:"startTime"`
	ResponseTime int64  `json:"responseTime"`
	Method       string `json:"method"`
}

func (l RPCLog) String() string {
	line, _ := json.Marshal(l)
	return string(line)
}

func LoggingInterceptor(logger logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		ctx, span := otel.GetTracerProvider().Tracer("gofr",
			trace.WithInstrumentationVersion("v0.1")).Start(ctx, info.FullMethod)
		start := time.Now()

		defer func() {
			l := RPCLog{
				ID:           trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
				StartTime:    start.Format("2006-01-02T15:04:05.999999999-07:00"),
				ResponseTime: time.Since(start).Microseconds(),
				Method:       info.FullMethod,
			}

			if logger != nil {
				logger.Infof("%s", l)
			}

			span.End()
		}()

		return handler(ctx, req)
	}
}
