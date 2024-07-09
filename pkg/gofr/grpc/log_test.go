package grpc

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

type contextKey string

const (
	id = "b00ff8de800911ec8f6502bfe7568078"
)

func TestRPCLog_String(t *testing.T) {
	l := RPCLog{
		ID:         "123",
		StartTime:  "2020-01-01T12:12:12",
		Method:     http.MethodGet,
		StatusCode: 0,
	}

	expLog := `{"id":"123","startTime":"2020-01-01T12:12:12","responseTime":0,"method":"GET","statusCode":0}`

	assert.Equal(t, expLog, l.String())
}

func TestLoggingInterceptor(t *testing.T) {
	var (
		err            = errors.New("DB error") //nolint:err113 // We are testing if a dynamic error would work
		key contextKey = "id"

		successHandler = func(context.Context, interface{}) (interface{}, error) {
			return "success", nil
		}

		errorHandler = func(context.Context, interface{}) (interface{}, error) {
			return nil, err
		}
	)

	serverInfo := &grpc.UnaryServerInfo{FullMethod: "/ExampleService/abc"}
	expLog := `"method":"ExampleService"`
	expLogWithTraceID := `\"id\":\"` + id + `"\",` + expLog

	tests := []struct {
		desc      string
		id        string
		handler   grpc.UnaryHandler
		expOutput interface{}
		err       error
		expLog    string
	}{
		{"handler returns successful response without traceID passed in context", "",
			successHandler, "success", nil, expLog},
		{"handler returns successful response with traceID passed in context", id,
			successHandler, "success", nil, expLogWithTraceID},
		{"handler returns error without traceID passed in context", "", errorHandler,
			nil, err, expLog},
		{"handler returns error with traceID passed in context", id,
			errorHandler, nil, err, expLogWithTraceID},
	}

	for i, tc := range tests {
		ctx := context.WithValue(context.Background(), key, tc.id)
		l := logging.NewMockLogger(logging.INFO)

		resp, err := LoggingInterceptor(l)(ctx, nil, serverInfo, tc.handler)

		assert.Equal(t, tc.expOutput, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
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

	for i, tc := range testCases {
		response := colorForGRPCCode(tc.code)

		assert.Equal(t, tc.colorCode, response, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestRPCLog_PrettyPrint(t *testing.T) {
	startTime := time.Now().String()

	log := testutil.StdoutOutputForFunc(func() {
		l := RPCLog{
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
