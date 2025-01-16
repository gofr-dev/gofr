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
	"google.golang.org/grpc/metadata"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
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
		err = errors.New("DB error") //nolint:err113 // We are testing if a dynamic error would work

		successHandler = func(context.Context, interface{}) (interface{}, error) {
			return "success", nil
		}

		errorHandler = func(context.Context, interface{}) (interface{}, error) {
			return nil, err
		}
	)

	mdWithoutTraceID := metadata.Pairs() // No traceId or spanId in metadata
	mdWithTraceID := metadata.Pairs("x-gofr-traceid", "traceid123", "x-gofr-spanid", "spanid123")

	expLog := `"method":"ExampleService"`
	expLogWithTraceID := `\"id\":\"traceid123\",\"method":"ExampleService"`

	tests := []struct {
		desc      string
		id        string
		md        metadata.MD
		handler   grpc.UnaryHandler
		expOutput interface{}
		err       error
		expLog    string
	}{
		{"handler returns successful response without traceID passed in metadata", "", mdWithoutTraceID,
			successHandler, "success", nil, expLog},
		{"handler returns successful response with traceID passed in metadata", "traceid123", mdWithTraceID,
			successHandler, "success", nil, expLogWithTraceID},
		{"handler returns error without traceID passed in metadata", "", mdWithoutTraceID,
			errorHandler, nil, err, expLog},
		{"handler returns error with traceID passed in metadata", "traceid123", mdWithTraceID,
			errorHandler, nil, err, expLogWithTraceID},
	}

	for i, tc := range tests {
		ctx := metadata.NewIncomingContext(context.Background(), tc.md)

		l := logging.NewMockLogger(logging.INFO)

		// Call the LoggingInterceptor with the context, passing metadata for each test case
		resp, err := LoggingInterceptor(l)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/ExampleService/abc"}, tc.handler)

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
