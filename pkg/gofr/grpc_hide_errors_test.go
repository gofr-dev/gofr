//go:build !gofr_nogrpc

package gofr

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

type grpcHideCase struct {
	desc     string
	err      error
	kept     bool // the interceptor returns err itself; otherwise the client gets wantCode and wantMsg
	wantCode codes.Code
	wantMsg  string
}

func grpcHideCases() []grpcHideCase {
	notFound := status.Error(codes.NotFound, "user 7 not found")

	return []grpcHideCase{
		{"raw error is replaced", errDriverDetail, false, codes.Internal, "Internal Server Error"},
		{"wrapped raw error is replaced", fmt.Errorf("lookup: %w", errDriverDetail), false, codes.Internal, "Internal Server Error"},
		{"status error is kept", notFound, true, 0, ""},
		{"wrapped status error is kept", fmt.Errorf("lookup: %w", notFound), true, 0, ""},
		{"context.Canceled is kept", context.Canceled, true, 0, ""},
		{"context.DeadlineExceeded is kept", context.DeadlineExceeded, true, 0, ""},
		{"wrapped context.Canceled keeps its code, not the wrapper text",
			fmt.Errorf("select where email='a@b' on 10.0.3.7: %w", context.Canceled), false,
			codes.Canceled, context.Canceled.Error()},
		{"wrapped context.DeadlineExceeded keeps its code, not the wrapper text",
			fmt.Errorf("select where email='a@b' on 10.0.3.7: %w", context.DeadlineExceeded), false,
			codes.DeadlineExceeded, context.DeadlineExceeded.Error()},
		{"nil stays nil", nil, true, 0, ""},
	}
}

func assertGRPCHidden(t *testing.T, tc grpcHideCase, err error) {
	t.Helper()

	if tc.kept {
		assert.Equal(t, tc.err, err, tc.desc)
		return
	}

	assert.Equal(t, tc.wantCode, status.Code(err), tc.desc)
	assert.Equal(t, tc.wantMsg, status.Convert(err).Message(), tc.desc)
}

func TestGRPC_HideErrorsUnaryInterceptor(t *testing.T) {
	interceptor := hideErrorsUnaryInterceptor()

	for _, tc := range grpcHideCases() {
		resp, err := interceptor(t.Context(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Get"},
			func(context.Context, any) (any, error) { return "resp", tc.err })

		assertGRPCHidden(t, tc, err)
		assert.Equal(t, "resp", resp, tc.desc)
	}
}

func TestGRPC_HideErrorsStreamInterceptor(t *testing.T) {
	for _, tc := range grpcHideCases() {
		var err error

		logs := testutil.StderrOutputForFunc(func() {
			interceptor := hideErrorsStreamInterceptor(logging.NewMockLogger(logging.DEBUG))

			err = interceptor(nil, incomingStream{ctx: t.Context()}, &grpc.StreamServerInfo{FullMethod: "/svc/Watch"},
				func(any, grpc.ServerStream) error { return tc.err })
		})

		assertGRPCHidden(t, tc, err)

		if !tc.kept {
			assert.Contains(t, logs, tc.err.Error(), "%s: the original error must be logged", tc.desc)
			assert.Contains(t, logs, "/svc/Watch", "%s: the log must name the method", tc.desc)
		}
	}
}

func TestGRPC_HideErrorsChain(t *testing.T) {
	for _, on := range []bool{false, true} {
		hideErrors(t, on)

		c := container.NewContainer(config.NewMockConfig(nil))

		g, err := newGRPCServer(c, testutil.GetFreePort(t), config.NewMockConfig(nil))
		require.NoError(t, err)

		want := defaultInterceptorCount
		if on {
			want++
		}

		assert.Len(t, g.interceptors, want, "unary chain, switch on=%v", on)
		assert.Len(t, g.streamInterceptors, want, "stream chain, switch on=%v", on)
	}
}

// TestGRPC_HideErrorsUnaryOutermost checks the interceptor sits outside observability: the client sees
// the generic status while the server log keeps the original error.
func TestGRPC_HideErrorsUnaryOutermost(t *testing.T) {
	hideErrors(t, true)

	var err error

	logs := testutil.StderrOutputForFunc(func() {
		c := container.NewContainer(config.NewMockConfig(nil))

		g, gErr := newGRPCServer(c, testutil.GetFreePort(t), config.NewMockConfig(nil))
		require.NoError(t, gErr)

		_, err = chainedUnary(t, g.interceptors)(t.Context(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Get"},
			func(context.Context, any) (any, error) { return nil, errDriverDetail })
	})

	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Equal(t, "Internal Server Error", status.Convert(err).Message())
	assert.Contains(t, logs, errDriverDetail.Error(), "observability must log the original error")
}

// TestGRPC_HideErrorsStreamOutermost checks the stream interceptor sits outside stream observability:
// observability logs the code of the original error (Unknown), not the replaced one (Internal).
func TestGRPC_HideErrorsStreamOutermost(t *testing.T) {
	hideErrors(t, true)

	var err error

	logs := testutil.StdoutOutputForFunc(func() {
		c := container.NewContainer(config.NewMockConfig(nil))

		g, gErr := newGRPCServer(c, testutil.GetFreePort(t), config.NewMockConfig(nil))
		require.NoError(t, gErr)

		err = chainedStream(g.streamInterceptors)(nil, incomingStream{ctx: t.Context()},
			&grpc.StreamServerInfo{FullMethod: "/svc/Watch", IsServerStream: true},
			func(any, grpc.ServerStream) error { return errDriverDetail })
	})

	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, logs, `"statusCode":2`, "observability must see the original (Unknown) error")
}

// chainedStream composes stream interceptors the way grpc.ChainStreamInterceptor does: the first is outermost.
func chainedStream(interceptors []grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		next := handler

		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor, inner := interceptors[i], next
			next = func(srv any, ss grpc.ServerStream) error { return interceptor(srv, ss, info, inner) }
		}

		return next(srv, ss)
	}
}

// chainedUnary composes interceptors the way grpc.ChainUnaryInterceptor does: the first is outermost.
func chainedUnary(t *testing.T, interceptors []grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	t.Helper()

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		next := handler

		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor, inner := interceptors[i], next
			next = func(ctx context.Context, req any) (any, error) { return interceptor(ctx, req, info, inner) }
		}

		return next(ctx, req)
	}
}
