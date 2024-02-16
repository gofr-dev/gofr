package google

import (
	"context"
	"errors"
	"fmt"
	"sync"

	gcPubSub "cloud.google.com/go/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

var errSubscriptionExistCheck = fmt.Errorf("unable to check the existence of subscription: ")

type Config struct {
	ProjectID        string
	SubscriptionName string
}

type googleClient struct {
	Config

	client       *gcPubSub.Client
	subscription *gcPubSub.Subscription
	logger       pubsub.Logger
	mu           *sync.Mutex
}

//nolint:revive // We do not want anyone using the client without initialization steps.
func New(conf Config, logger pubsub.Logger) *googleClient {
	client, err := gcPubSub.NewClient(context.Background(), conf.ProjectID)
	if err != nil {
		return &googleClient{
			Config: conf,
		}
	}

	// create subscription

	return &googleClient{
		Config: conf,
		client: client,
		logger: logger,
	}
}

func (g *googleClient) Publish(ctx context.Context, topic string, message []byte) error {
	t := g.client.Topic(topic)

	if ok, err := t.Exists(ctx); !ok || err != nil {
		_, err := g.client.CreateTopic(ctx, topic)
		if err != nil {
			return err
		}
	}

	result := t.Publish(ctx, &gcPubSub.Message{
		Data: message,
	})

	_, err := result.Get(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (g *googleClient) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	var m *pubsub.Message

	t := g.client.Topic(topic)
	if ok, err := t.Exists(ctx); !ok || err != nil {
		_, err := g.client.CreateTopic(ctx, topic)
		if err != nil {
			return nil, err
		}
	}

	if g.subscription == nil {
		g.subscription = g.client.Subscription(g.SubscriptionName)
	}

	ok, err := g.subscription.Exists(context.Background())
	if err != nil {
		g.logger.Errorf(errSubscriptionExistCheck.Error() + err.Error())

		return nil, errors.New(errSubscriptionExistCheck.Error() + err.Error())
	}

	if !ok {
		g.subscription, err = g.client.CreateSubscription(ctx, g.SubscriptionName, gcPubSub.SubscriptionConfig{
			Topic: t,
		})

		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)

	err = g.subscription.Receive(ctx, func(_ context.Context, msg *gcPubSub.Message) {
		defer cancel()

		m = &pubsub.Message{
			Topic:    topic,
			Value:    msg.Data,
			MetaData: msg.Attributes,
		}

		msg.Ack()
	})

	if err != nil {
		g.logger.Errorf("Error getting a message: %s", err.Error())

		return nil, err
	}

	return m, nil
}

func (g *googleClient) Commit(ctx context.Context, msg pubsub.Message) error {
	return nil
}
