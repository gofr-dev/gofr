package nats

import "context"

//go:generate mockgen -destination=mock_metrics.go -package=nats -source=./metrics.go

type Metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}
