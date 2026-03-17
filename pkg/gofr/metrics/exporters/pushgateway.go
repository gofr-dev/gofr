package exporters

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

// PushGateway pushes metrics from the default Prometheus registry to a Pushgateway.
type PushGateway struct {
	pusher *push.Pusher
	logger logger
}

type logger interface {
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
}

// NewPushGateway creates a PushGateway pusher for the given URL and job name.
func NewPushGateway(url, jobName string, l logger) *PushGateway {
	pusher := push.New(url, jobName).Gatherer(prometheus.DefaultGatherer)

	return &PushGateway{pusher: pusher, logger: l}
}

// Push pushes all metrics to the pushgateway. Should be called on shutdown.
func (p *PushGateway) Push(ctx context.Context) error {
	p.logger.Logf("Pushing metrics to Pushgateway")

	if err := p.pusher.PushContext(ctx); err != nil {
		p.logger.Errorf("failed to push metrics to Pushgateway: %v", err)
		return err
	}

	return nil
}
