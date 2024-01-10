package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
	"google.golang.org/grpc"
)

func TestNewGRPCServer(t *testing.T) {
	c := container.Container{
		Logger: logging.NewLogger(logging.DEBUG),
	}

	g := newGRPCServer(&c, 9999)
	if g == nil {
		t.Errorf("FAILED, Expected: a non nil value, Got: %v", g)
	}
}

func TestGRPC_ServerRun(t *testing.T) {
	testCases := []struct {
		desc        string
		grcpServer  *grpc.Server
		port        int
		expectedLog string
	}{
		{
			desc:        "net.Listen() error",
			grcpServer:  nil,
			port:        99999,
			expectedLog: "error in starting grpc server",
		},
		{
			desc:        "server.Serve() error",
			grcpServer:  new(grpc.Server),
			port:        10000,
			expectedLog: "error in starting grpc serve",
		},
	}

	for _, tc := range testCases {
		f := func() {
			c := &container.Container{
				Logger: logging.NewLogger(logging.INFO),
			}

			g := &grpcServer{
				server: tc.grcpServer,
				port:   tc.port,
			}

			g.Run(c)
		}

		out := testutil.StderrOutputForFunc(f)

		assert.Contains(t, out, tc.expectedLog)
	}
}
