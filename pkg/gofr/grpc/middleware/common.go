package middleware

import (
	"context"

	"google.golang.org/grpc"
)

const headerParts = 2

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
