package nats

import (
	"context"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// PubSubWrapper adapts Client to pubsub.JetStreamClient.
type PubSubWrapper struct {
	Client *Client
}

func (w *PubSubWrapper) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	return w.Client.Query(ctx, query, args...)
}

// Publish publishes a message to a topic.
func (w *PubSubWrapper) Publish(ctx context.Context, topic string, message []byte) error {
	return w.Client.Publish(ctx, topic, message)
}

// Subscribe subscribes to a topic and returns a single message.
func (w *PubSubWrapper) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	return w.Client.Subscribe(ctx, topic)
}

// CreateTopic creates a new topic (stream) in NATS jStream.
func (w *PubSubWrapper) CreateTopic(ctx context.Context, name string) error {
	return w.Client.CreateTopic(ctx, name)
}

// DeleteTopic deletes a topic (stream) in NATS jStream.
func (w *PubSubWrapper) DeleteTopic(ctx context.Context, name string) error {
	return w.Client.DeleteTopic(ctx, name)
}

// Close closes the Client.
func (w *PubSubWrapper) Close() error {
	ctx := context.Background()
	return w.Client.Close(ctx)
}

// Health returns the health status of the Client.
func (w *PubSubWrapper) Health() datasource.Health {
	return w.Client.Health()
}

// Connect establishes a connection to NATS.
func (w *PubSubWrapper) Connect() {
	if w.Client.connManager != nil && w.Client.connManager.Health().Status == datasource.StatusUp {
		w.Client.logger.Log("NATS connection already established")

		return
	}

	err := w.Client.Connect()
	if err != nil {
		w.Client.logger.Errorf("PubSubWrapper: Error connecting to NATS: %v", err)
	}
}

// UseLogger sets the logger for the NATS client.
func (w *PubSubWrapper) UseLogger(logger any) {
	w.Client.UseLogger(logger)
}

// UseMetrics sets the metrics for the NATS client.
func (w *PubSubWrapper) UseMetrics(metrics any) {
	w.Client.UseMetrics(metrics)
}

// UseTracer sets the tracer for the NATS client.
func (w *PubSubWrapper) UseTracer(tracer any) {
	w.Client.UseTracer(tracer)
}
