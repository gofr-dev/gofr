package gofr

import (
	"context"
	"encoding/json"
	"strings"

	"net"
	"strconv"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

type GRPC struct {
	server *grpc.Server
	Port   int
}

// Server return an object of grpc server
func (g *GRPC) Server() *grpc.Server {
	return g.server
}

// NewGRPCServer creates a gRPC server instance with OpenTelemetry tracing, OpenCensus stats handling,
// unary interceptors for tracing and recovery, and a custom logging interceptor.
func NewGRPCServer() *grpc.Server {
	return grpc.NewServer(
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider())),
			grpc_recovery.UnaryServerInterceptor(),
			LoggingInterceptor(log.NewLogger()),
		)))
}

// Start initializes and starts the gRPC server on the specified port.
// It logs the server address and attempts to establish a TCP listener. If successful, it serves incoming connections.
// If any errors occur during the process,
// it logs the specific error message along with the server address, ensuring proper error handling and logging.
func (g *GRPC) Start(logger log.Logger) {
	addr := ":" + strconv.Itoa(g.Port)

	logger.Infof("starting grpc server at %s", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Errorf("error in starting grpc server at %s: %s", addr, err)
		return
	}

	if err := g.server.Serve(listener); err != nil {
		logger.Errorf("error in starting grpc server at %s: %s", addr, err)
		return
	}
}

type RPCLog struct {
	ID           string `json:"correlationId"`
	StartTime    string `json:"startTime"`
	ResponseTime int64  `json:"responseTime"`
	Duration     int64  `json:"duration"`
	Method       string `json:"method"`
	URI          string `json:"uri"`
}

// String converts an RPCLog instance to its JSON representation and returns it as a string.
func (l *RPCLog) String() string {
	line, _ := json.Marshal(l)
	return string(line)
}

// LoggingInterceptor is a gRPC unary server interceptor that captures and logs information about incoming requests and
// their handling duration.
// It extracts the correlation ID from the context or generates a new one if unavailable, records request details
// (such as method and URI),
// measures the duration of request processing, and logs this information using the provided logger.
// The interceptor then proceeds to handle the request and returns the response.
func LoggingInterceptor(logger log.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		start := time.Now()

		var correlationID string

		cID := types.GetCorrelationIDFromContext(ctx)
		if err = cID.Validate(); err != nil {
			cID = types.GenerateCorrelationID(ctx)
		}

		ctx, err = cID.SetInContext(ctx)
		if err != nil {
			return nil, err
		}

		correlationID = cID.String()

		fullMethod := strings.Split(info.FullMethod, "/")
		uri := "/" + strings.Join(fullMethod[2:], "/")

		defer func() {
			l := RPCLog{
				ID:        correlationID,
				StartTime: start.Format("2006-01-02T15:04:05.999999999-07:00"),
				Duration:  time.Since(start).Microseconds(),
				Method:    fullMethod[1],
				URI:       uri,
			}

			if logger != nil {
				logger.Log(&l)
			}
		}()

		return handler(ctx, req)
	}
}
