package nats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

//go:generate mockgen -destination=mock_jetstream.go -package=nats github.com/nats-io/nats.go/jetstream JetStream,Stream,Consumer,Msg,MessageBatch

const (
	ctxCloseTimeout = 5 * time.Second
)

type ConnectionManager struct {
	conn      ConnInterface
	jetStream jetstream.JetStream
	config    *Config
	logger    pubsub.Logger
}

func (cm *ConnectionManager) JetStream() jetstream.JetStream {
	return cm.jetStream
}

// natsConnWrapper wraps a nats.Conn to implement the ConnInterface.
type natsConnWrapper struct {
	*nats.Conn
}

func (w *natsConnWrapper) Status() nats.Status {
	return w.Conn.Status()
}

func (w *natsConnWrapper) Close() {
	w.Conn.Close()
}

func (w *natsConnWrapper) NatsConn() *nats.Conn {
	return w.Conn
}

func NewConnectionManager(cfg *Config, logger pubsub.Logger) *ConnectionManager {
	return &ConnectionManager{
		config: cfg,
		logger: logger,
	}
}

func (cm *ConnectionManager) Connect() error {
	nc, err := cm.createNATSConnection()
	if err != nil {
		return err
	}

	js, err := cm.createJetStreamContext(nc)
	if err != nil {
		nc.Close()
		return err
	}

	cm.conn = &natsConnWrapper{nc}
	cm.jetStream = js

	return nil
}

func (cm *ConnectionManager) createNATSConnection() (*nats.Conn, error) {
	opts := []nats.Option{nats.Name("GoFr NATS JetStreamClient")}
	if cm.config.CredsFile != "" {
		opts = append(opts, nats.UserCredentials(cm.config.CredsFile))
	}

	nc, err := nats.Connect(cm.config.Server, opts...)
	if err != nil {
		cm.logger.Errorf("failed to connect to NATS server at %v: %v", cm.config.Server, err)
		return nil, err
	}

	return nc, nil
}

func (cm *ConnectionManager) createJetStreamContext(nc *nats.Conn) (jetstream.JetStream, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		cm.logger.Errorf("failed to create JetStream context: %v", err)
		return nil, err
	}

	return js, nil
}

func (cm *ConnectionManager) Close(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, ctxCloseTimeout)
	defer cancel()

	if cm.conn != nil {
		cm.conn.Close()
	}
}

func (cm *ConnectionManager) Publish(ctx context.Context, subject string, message []byte, metrics Metrics) error {
	metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", subject)

	if err := cm.validateJetStream(subject); err != nil {
		return err
	}

	_, err := cm.jetStream.Publish(ctx, subject, message)
	if err != nil {
		cm.logger.Errorf("failed to publish message to NATS JetStream: %v", err)
		return err
	}

	metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "subject", subject)
	return nil
}

func (cm *ConnectionManager) validateJetStream(subject string) error {
	if cm.jetStream == nil || subject == "" {
		err := errJetStreamNotConfigured
		cm.logger.Error(err.Error())
		return err
	}
	return nil
}

func (cm *ConnectionManager) Health() datasource.Health {
	if cm.conn == nil {
		return datasource.Health{
			Status: datasource.StatusDown,
		}
	}

	status := cm.conn.Status()
	if status == nats.CONNECTED {
		return datasource.Health{
			Status: datasource.StatusUp,
			Details: map[string]interface{}{
				"server": cm.config.Server,
			},
		}
	}

	return datasource.Health{
		Status: datasource.StatusDown,
		Details: map[string]interface{}{
			"server": cm.config.Server,
		},
	}
}
