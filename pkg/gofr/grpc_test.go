package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestNewGRPCServer(t *testing.T) {
	testConf := testConfig{
		"LOG_LEVEL": "DEBUG",
	}

	c := container.Container{
		Logger: logging.NewLogger(testConf),
	}

	g := newGRPCServer(&c, 9999)

	assert.NotNil(t, g, "TEST Failed.\n")
}

func TestGRPC_ServerRun(t *testing.T) {
	testConf := testConfig{
		"LOG_LEVEL": "INFO",
	}

	testCases := []struct {
		desc       string
		grcpServer *grpc.Server
		port       int
		expLog     string
	}{
		{"net.Listen() error", nil, 99999, "error in starting grpc server"},
		{"server.Serve() error", new(grpc.Server), 10000, "error in starting grpc server"},
	}

	for i, tc := range testCases {
		f := func() {
			c := &container.Container{
				Logger: logging.NewLogger(testConf),
			}

			g := &grpcServer{
				server: tc.grcpServer,
				port:   tc.port,
			}

			g.Run(c)
		}

		out := testutil.StderrOutputForFunc(f)

		assert.Contains(t, out, tc.expLog, "TEST[%d], Failed.\n", i)
	}
}

type testConfig map[string]string

func (c testConfig) Get(key string) string {
	return c[key]
}

func (c testConfig) GetOrDefault(key, defaultValue string) string {
	if value, ok := c[key]; ok {
		return value
	}
	return defaultValue
}
