package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

const (
	consumeMessageDelay = 100 * time.Millisecond
)

type SubscriptionManager struct {
	subscriptions map[string]*subscription
	subMu         sync.Mutex
	topicBuffers  map[string]chan *pubsub.Message
	bufferMu      sync.RWMutex
	bufferSize    int
}

type subscription struct {
	cancel context.CancelFunc
}

func newSubscriptionManager(bufferSize int) *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]*subscription),
		topicBuffers:  make(map[string]chan *pubsub.Message),
		bufferSize:    bufferSize,
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

	sm.subMu.Lock()

	_, exists := sm.subscriptions[topic]
	if !exists {
		cons, err := sm.createOrUpdateConsumer(ctx, js, topic, cfg)
		if err != nil {
			sm.subMu.Unlock()
			return nil, err
		}

		subCtx, cancel := context.WithCancel(context.Background())
		sm.subscriptions[topic] = &subscription{cancel: cancel}

		buffer := sm.getOrCreateBuffer(topic)
		go sm.consumeMessages(subCtx, cons, topic, buffer, cfg, logger)
	}

	sm.subMu.Unlock()

	buffer := sm.getOrCreateBuffer(topic)

	select {
	case msg := <-buffer:
		metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic)
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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

func (sm *SubscriptionManager) getOrCreateBuffer(topic string) chan *pubsub.Message {
	sm.bufferMu.Lock()
	defer sm.bufferMu.Unlock()

	if buffer, exists := sm.topicBuffers[topic]; exists {
		return buffer
	}

	buffer := make(chan *pubsub.Message, sm.bufferSize)
	sm.topicBuffers[topic] = buffer

	return buffer
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
		AckWait:       30 * time.Second,
	})

	return cons, err
}

func (*SubscriptionManager) consumeMessages(
	ctx context.Context,
	cons jetstream.Consumer,
	topic string,
	buffer chan *pubsub.Message,
	cfg *Config,
	logger pubsub.Logger) {
	// TODO: propagate errors to caller
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(cfg.MaxWait))
			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					logger.Errorf("Error fetching messages for topic %s: %v", topic, err)
				}

				time.Sleep(consumeMessageDelay)

				continue
			}

			for msg := range msgs.Messages() {
				pubsubMsg := pubsub.NewMessage(ctx)
				pubsubMsg.Topic = topic
				pubsubMsg.Value = msg.Data()
				pubsubMsg.MetaData = msg.Headers()
				pubsubMsg.Committer = &natsCommitter{msg: msg}

				select {
				case buffer <- pubsubMsg:
					// Message sent successfully
				default:
					logger.Logf("Message buffer is full for topic %s. Consider increasing buffer size or processing messages faster.", topic)
				}
			}

			if err := msgs.Error(); err != nil {
				logger.Errorf("Error in message batch for topic %s: %v", topic, err)
			}
		}
	}
}

func (sm *SubscriptionManager) Close() {
	sm.subMu.Lock()
	for _, sub := range sm.subscriptions {
		sub.cancel()
	}

	sm.subscriptions = make(map[string]*subscription)
	sm.subMu.Unlock()

	sm.bufferMu.Lock()
	for _, buffer := range sm.topicBuffers {
		close(buffer)
	}

	sm.topicBuffers = make(map[string]chan *pubsub.Message)

	sm.bufferMu.Unlock()
}
