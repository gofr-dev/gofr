package middleware

import (
	"context"

	"google.golang.org/grpc"
)

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
