package grpc

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

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
	assert.Equal(t, "00000000000000000000000000000000", getTraceID(context.TODO()))
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
