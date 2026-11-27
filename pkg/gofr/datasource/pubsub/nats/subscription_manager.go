package nats

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// SubscriptionManager delivers one message per Subscribe call, fetching from
// JetStream only when the application is ready to handle the next message.
// The broker starts the AckWait timer when a message is delivered, so messages
// fetched ahead into a buffer would sit past AckWait and get redelivered while
// their first run is still queued.
type SubscriptionManager struct {
	consumers     map[string]jetstream.Consumer
	consumersLock sync.Mutex
}

func newSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		consumers: make(map[string]jetstream.Consumer),
	}
}

func (sm *SubscriptionManager) Subscribe(
	ctx context.Context,
	topic string,
	js jetstream.JetStream,
	cfg *Config,
	logger pubsub.Logger,
	metrics Metrics) (*pubsub.Message, error) {
	metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic)

	if err := sm.validateSubscribePrerequisites(js, cfg); err != nil {
		return nil, err
	}

	cons, err := sm.consumerFor(ctx, js, topic, cfg)
	if err != nil {
		return nil, err
	}

	msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(cfg.MaxWait))
	if err != nil {
		logger.Errorf("Error fetching messages for topic %s: %v", topic, err)

		return nil, err
	}

	return sm.messageFromBatch(ctx, msgs, topic, logger, metrics)
}

func (*SubscriptionManager) validateSubscribePrerequisites(js jetstream.JetStream, cfg *Config) error {
	if js == nil {
		return errJetStreamNotConfigured
	}

	if cfg.Consumer == "" {
		return errConsumerNotProvided
	}

	return nil
}

// consumerFor returns the durable consumer for the topic, creating it on first use.
func (sm *SubscriptionManager) consumerFor(
	ctx context.Context, js jetstream.JetStream, topic string, cfg *Config) (jetstream.Consumer, error) {
	sm.consumersLock.Lock()
	defer sm.consumersLock.Unlock()

	if cons, exists := sm.consumers[topic]; exists {
		return cons, nil
	}

	cons, err := sm.createOrUpdateConsumer(ctx, js, topic, cfg)
	if err != nil {
		return nil, err
	}

	sm.consumers[topic] = cons

	return cons, nil
}

func (*SubscriptionManager) createOrUpdateConsumer(
	ctx context.Context, js jetstream.JetStream, topic string, cfg *Config) (jetstream.Consumer, error) {
	consumerName := fmt.Sprintf("%s_%s", cfg.Consumer, strings.ReplaceAll(topic, ".", "_"))
	cons, err := js.CreateOrUpdateConsumer(ctx, cfg.Stream.Stream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: topic,
		MaxDeliver:    cfg.Stream.MaxDeliver,
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckWait:       defaultAckWait,
	})

	return cons, err
}

// messageFromBatch returns the fetched message, or nil when nothing arrived
// before the fetch wait expired.
func (sm *SubscriptionManager) messageFromBatch(
	ctx context.Context, msgs jetstream.MessageBatch, topic string, logger pubsub.Logger, metrics Metrics) (*pubsub.Message, error) {
	for msg := range msgs.Messages() {
		metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic)

		return sm.createPubSubMessage(ctx, msg, topic), nil
	}

	if err := msgs.Error(); err != nil {
		logger.Errorf("Error in message batch for topic %s: %v", topic, err)

		return nil, err
	}

	return nil, nil
}

func (*SubscriptionManager) createPubSubMessage(ctx context.Context, msg jetstream.Msg, topic string) *pubsub.Message {
	spanCtx, span := startSubscribeSpan(ctx, otel.GetTracerProvider().Tracer(tracerName), topic, msg.Headers())

	pubsubMsg := pubsub.NewMessage(spanCtx)
	pubsubMsg.Topic = topic
	pubsubMsg.Value = msg.Data()
	pubsubMsg.MetaData = msg.Headers()
	pubsubMsg.Committer = &natsCommitter{msg: msg, span: span}

	return pubsubMsg
}

func (sm *SubscriptionManager) Close() {
	sm.consumersLock.Lock()
	defer sm.consumersLock.Unlock()

	sm.consumers = make(map[string]jetstream.Consumer)
}
