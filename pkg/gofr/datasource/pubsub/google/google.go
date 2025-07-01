// Package google provides a client for interacting with Google Cloud Pub/Sub.This package facilitates interaction with
// Google Cloud Pub/Sub, allowing publishing and subscribing to topics, managing subscriptions, and handling messages.
package google

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	gcPubSub "cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel"
	"google.golang.org/api/iterator"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

var (
	errProjectIDNotProvided    = errors.New("google project id not provided")
	errSubscriptionNotProvided = errors.New("subscription name not provided")
	errClientNotConnected      = errors.New("google pubsub client is not connected")
	errTopicName               = errors.New("empty topic name")
)

const (
	defaultRetryInterval = 10 * time.Second
	messageBufferSize    = 100
)

type Config struct {
	ProjectID        string
	SubscriptionName string
}

type googleClient struct {
	Config

	client      Client
	logger      pubsub.Logger
	metrics     Metrics
	receiveChan map[string]chan *pubsub.Message
	subStarted  map[string]struct{}
	mu          sync.RWMutex
}

const (
	defaultQueryTimeout = 30 * time.Second
	defaultMessageLimit = 10
)

//nolint:revive // We do not want anyone using the client without initialization steps.
func New(conf Config, logger pubsub.Logger, metrics Metrics) *googleClient {
	err := validateConfigs(&conf)
	if err != nil {
		logger.Errorf("could not configure google pubsub, error: %v", err)

		return nil
	}

	logger.Debugf("connecting to google pubsub client with projectID '%s' and subscriptionName '%s", conf.ProjectID, conf.SubscriptionName)

	var client googleClient
	client.Config = conf
	client.logger = logger
	client.metrics = metrics
	client.receiveChan = make(map[string]chan *pubsub.Message)
	client.subStarted = make(map[string]struct{})
	client.mu = sync.RWMutex{}

	gClient, err := connect(conf, logger)
	if err != nil {
		go retryConnect(conf, logger, &client)

		return &client
	}

	client.client = gClient

	return &client
}

func connect(conf Config, logger pubsub.Logger) (*gcPubSub.Client, error) {
	client, err := gcPubSub.NewClient(context.Background(), conf.ProjectID)
	if err != nil {
		logger.Errorf("could not create Google PubSub client, error: %v", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRetryInterval)
	defer cancel()

	it := client.Topics(ctx)

	_, err = it.Next()
	if err != nil {
		if errors.Is(err, iterator.Done) {
			logger.Debugf("no topics found in Google PubSub")
			return client, nil
		}

		logger.Errorf("google pubsub connection validation failed, error: %v", err)

		return nil, err
	}

	logger.Logf("connected to google pubsub client, projectID: %s", client.Project())

	return client, nil
}

func (g *googleClient) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "publish-gcp")
	defer span.End()

	g.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	t, err := g.getTopic(ctx, topic)
	if err != nil {
		g.logger.Errorf("could not create topic '%s', error: %v", topic, err)

		return err
	}

	start := time.Now()
	result := t.Publish(ctx, &gcPubSub.Message{
		Data:        message,
		PublishTime: time.Now(),
	})
	end := time.Since(start)

	_, err = result.Get(ctx)
	if err != nil {
		g.logger.Errorf("error publishing to google topic '%s', error: %v", topic, err)

		return err
	}

	g.logger.Debug(&pubsub.Log{
		Mode:          "PUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(message),
		Topic:         topic,
		Host:          g.ProjectID,
		PubSubBackend: "GCP",
		Time:          end.Microseconds(),
	})

	g.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

func (g *googleClient) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	var end time.Duration

	if g.client == nil {
		return nil, nil
	}

	if !g.isConnected() {
		time.Sleep(defaultRetryInterval)

		return nil, errClientNotConnected
	}

	spanCtx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "gcp-subscribe")
	defer span.End()

	g.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_total_count", "topic", topic, "subscription_name", g.Config.SubscriptionName)

	if _, ok := g.subStarted[topic]; !ok {
		t, err := g.getTopic(spanCtx, topic)
		if err != nil {
			return nil, err
		}

		subscription, err := g.getSubscription(spanCtx, t)
		if err != nil {
			return nil, err
		}

		start := time.Now()

		processMessage := func(ctx context.Context, msg *gcPubSub.Message) {
			m := pubsub.NewMessage(ctx)
			end = time.Since(start)

			m.Topic = topic
			m.Value = msg.Data
			m.MetaData = msg.Attributes
			m.Committer = newGoogleMessage(msg)

			g.mu.Lock()
			defer g.mu.Unlock()

			g.receiveChan[topic] <- m
		}

		// initialize the channel before we can start receiving on it
		g.mu.Lock()
		g.receiveChan[topic] = make(chan *pubsub.Message)
		g.mu.Unlock()

		go func() {
			err = subscription.Receive(ctx, processMessage)
			if err != nil {
				g.logger.Errorf("error getting a message from google: %s", err.Error())
			}
		}()

		g.subStarted[topic] = struct{}{}
	}

	select {
	case m := <-g.receiveChan[topic]:
		g.metrics.IncrementCounter(spanCtx, "app_pubsub_subscribe_success_count", "topic", topic, "subscription_name",
			g.Config.SubscriptionName)

		g.logger.Debug(&pubsub.Log{
			Mode:          "SUB",
			CorrelationID: span.SpanContext().TraceID().String(),
			MessageValue:  string(m.Value),
			Topic:         topic,
			Host:          g.Config.ProjectID,
			PubSubBackend: "GCP",
			Time:          end.Microseconds(),
		})

		return m, nil
	case <-ctx.Done():
		return nil, nil
	}
}

func (g *googleClient) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if !g.isConnected() {
		return nil, errClientNotConnected
	}

	if query == "" {
		return nil, errTopicName
	}

	timeout, limit := parseQueryArgs(args...)

	// Get topic and subscription
	topic, err := g.getTopic(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}

	subscription, err := g.getQuerySubscription(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	msgChan := make(chan []byte, messageBufferSize)
	queryCtx, cancel := context.WithTimeout(ctx, timeout)

	defer cancel()

	// Start receiving messages
	go func() {
		defer close(msgChan)

		receiveCtx, receiveCancel := context.WithTimeout(queryCtx, timeout)
		defer receiveCancel()

		err := subscription.Receive(receiveCtx, func(_ context.Context, msg *gcPubSub.Message) {
			defer msg.Ack()

			select {
			case msgChan <- msg.Data:
			case <-receiveCtx.Done():
				return
			default:
				// Channel might be full, try non-blocking send
				g.logger.Debugf("Query: message channel full for topic %s", query)
			}
		})

		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			g.logger.Debugf("Query: receive ended for topic %s: %v", query, err)
		}
	}()

	// Collect messages
	return g.collectMessages(queryCtx, msgChan, limit), nil
}

func (g *googleClient) getTopic(ctx context.Context, topic string) (*gcPubSub.Topic, error) {
	if g.client == nil {
		return nil, errClientNotConnected
	}

	t := g.client.Topic(topic)

	// check if topic exists, if not create the topic
	if ok, err := t.Exists(ctx); !ok || err != nil {
		_, errTopicCreate := g.client.CreateTopic(ctx, topic)
		if errTopicCreate != nil {
			return nil, errTopicCreate
		}
	}

	return t, nil
}

func (g *googleClient) getSubscription(ctx context.Context, topic *gcPubSub.Topic) (*gcPubSub.Subscription, error) {
	if g.client == nil {
		return nil, errClientNotConnected
	}

	subscription := g.client.Subscription(g.SubscriptionName + "-" + topic.ID())

	// check if subscription already exists or not
	ok, err := subscription.Exists(context.Background())
	if err != nil {
		g.logger.Errorf("unable to check the existence of subscription, error: %v", err.Error())

		return nil, err
	}

	// if subscription is not present, create a new
	if !ok {
		subscription, err = g.client.CreateSubscription(ctx, g.SubscriptionName+"-"+topic.ID(), gcPubSub.SubscriptionConfig{
			Topic: topic,
		})

		if err != nil {
			return nil, err
		}
	}

	return subscription, nil
}

func (g *googleClient) DeleteTopic(ctx context.Context, name string) error {
	if g.client == nil {
		return errClientNotConnected
	}

	err := g.client.Topic(name).Delete(ctx)
	if err != nil && strings.Contains(err.Error(), "Topic not found") {
		return nil
	}

	return err
}

func (g *googleClient) CreateTopic(ctx context.Context, name string) error {
	if g.client == nil {
		return errClientNotConnected
	}

	_, err := g.client.CreateTopic(ctx, name)
	if err != nil && strings.Contains(err.Error(), "Topic already exists") {
		return nil
	}

	return err
}

func (g *googleClient) Close() error {
	if g.client != nil {
		return g.client.Close()
	}

	for _, c := range g.receiveChan {
		close(c)
	}

	return nil
}

func retryConnect(conf Config, logger pubsub.Logger, g *googleClient) {
	for {
		client, err := connect(conf, logger)
		if err == nil {
			g.mu.Lock()
			g.client = client
			g.mu.Unlock()

			logger.Logf("connected to google pubsub client, projectID: %s", conf.ProjectID)

			return
		}

		logger.Errorf("could not connect to Google PubSub, error: %v", err)

		time.Sleep(defaultRetryInterval)
	}
}
