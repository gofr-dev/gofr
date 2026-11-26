//go:build !gofr_nogrpc

package gofr

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gofr.dev/pkg/gofr/internal/errhide"
	"gofr.dev/pkg/gofr/logging"
)

// errGRPCHidden replaces an error whose text HIDE_INTERNAL_ERRORS keeps away from gRPC clients.
var errGRPCHidden = status.Error(codes.Internal, errhide.Message)

// hideErrorsUnaryInterceptor replaces a handler error that is not a gRPC status error with a generic
// Internal status. It runs outermost, so the observability interceptor inside it still logs the
// original error.
func hideErrorsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		_, err = hideGRPCError(err)

		return resp, err
	}
}

// hideErrorsStreamInterceptor is the stream counterpart of hideErrorsUnaryInterceptor. The stream
// observability interceptor logs only the status code, so this one logs the original error itself.
func hideErrorsStreamInterceptor(logger logging.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)

		replaced, out := hideGRPCError(err)
		if replaced {
			logger.Errorf("error while handling gRPC stream to method %q: %v", info.FullMethod, err)
		}

		return out
	}
}

// hideGRPCError keeps errors whose text gRPC clients are meant to see: status errors, also when
// wrapped (status.FromError unwraps them, and the client then sees the wrapper's full text), and a
// bare context.Canceled or context.DeadlineExceeded, which gRPC sends with its own code. A wrapped
// context error keeps its code but not the wrapper's text, which can carry query or address detail.
// Everything else becomes a generic Internal status. replaced reports whether out differs from err.
func hideGRPCError(err error) (replaced bool, out error) {
	if err == nil {
		return false, nil
	}

	if _, ok := status.FromError(err); ok {
		return false, err
	}

	for _, ctxErr := range []error{context.Canceled, context.DeadlineExceeded} {
		if !errors.Is(err, ctxErr) {
			continue
		}

		if errors.Unwrap(err) == nil { // the bare context error is sent as is
			return false, err
		}

		return true, status.FromContextError(ctxErr).Err()
	}

	return true, errGRPCHidden
}
