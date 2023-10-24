package gofr

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

const correlationID = "b00ff8de800911ec8f6502bfe7568078"

func TestRPCLog_String(t *testing.T) {
	l := RPCLog{
		ID:        "123",
		StartTime: "2020-01-01T12:12:12",
		Duration:  100,
		Method:    http.MethodGet,
		URI:       "getEmployee",
	}

	expected := `{"correlationId":"123","startTime":"2020-01-01T12:12:12","responseTime":0,"duration":100,"method":"GET","uri":"getEmployee"}`
	got := l.String()

	assert.Equal(t, expected, got)
}

func TestGRPC_Server(t *testing.T) {
	tcs := []struct {
		input *grpc.Server
	}{
		{nil},
		{new(grpc.Server)},
	}

	for _, tc := range tcs {
		g := new(GRPC)
		g.server = tc.input

		if g.Server() != tc.input {
			t.Errorf("FAILED, Expected: %v, Got: %v", tc.input, g.Server())
		}
	}
}

func TestNewGRPCServer(t *testing.T) {
	g := NewGRPCServer()
	if g == nil {
		t.Errorf("FAILED, Expected: a non nil value, Got: %v", g)
	}
}

func TestGRPC_Start(t *testing.T) {
	type fields struct {
		server *grpc.Server
		Port   int
	}

	type args struct {
		logger log.Logger
	}

	tests := []struct {
		name        string
		fields      fields
		args        args
		expectedLog string
	}{
		{
			name:        "net.Listen() error",
			fields:      fields{server: nil, Port: 99999},
			expectedLog: "error in starting grpc server",
		},
		{
			name:        "server.Serve() error",
			fields:      fields{server: new(grpc.Server), Port: 10000},
			expectedLog: "error in starting grpc server",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			b := new(bytes.Buffer)
			tt.args.logger = log.NewMockLogger(b)

			g := &GRPC{
				server: tt.fields.server,
				Port:   tt.fields.Port,
			}

			g.Start(tt.args.logger)

			if !strings.Contains(b.String(), "error in starting grpc server") {
				t.Errorf("FAILED, Expected: `%v` in logs", "error in starting grpc server")
			}
		})
	}
}

func TestLoggingInterceptor(t *testing.T) {
	b := new(bytes.Buffer)
	l := log.NewMockLogger(b)

	successHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}
	errorHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.DB{}
	}

	serverInfo := &grpc.UnaryServerInfo{FullMethod: "/ExampleService/abc"}
	expLog := `"method":"ExampleService"`
	expLogWithTraceID := `\"correlationId\":\"` + correlationID + `"\",` + expLog

	tests := []struct {
		desc          string
		correlationID string
		handler       grpc.UnaryHandler
		expOutput     interface{}
		err           error
		expLog        string
	}{
		{"handler returns successful response without traceID passed in context", "",
			successHandler, "success", nil, expLog},
		{"handler returns successful response with traceID passed in context", correlationID,
			successHandler, "success", nil, expLogWithTraceID},
		{"handler returns error without traceID passed in context", "", errorHandler,
			nil, errors.DB{}, expLog},
		{"handler returns error with traceID passed in context", correlationID,
			errorHandler, nil, errors.DB{}, expLogWithTraceID},
	}

	for i, tc := range tests {
		ctx := context.WithValue(context.Background(), middleware.CorrelationIDKey, tc.correlationID)

		resp, err := LoggingInterceptor(l)(ctx, nil, serverInfo, tc.handler)

		if !strings.Contains(b.String(), expLog) {
			t.Errorf("expected %s to be present in log.\nGot: %s", expLog, b.String())
		}

		assert.Equal(t, tc.expOutput, resp, "TEST[%d] failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.err, err, "TEST[%d] failed.\n%s", i, tc.desc)
	}
}
