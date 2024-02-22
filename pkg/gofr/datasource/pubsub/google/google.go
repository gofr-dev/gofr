package google

import (
	"context"
	"fmt"

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

	client *gcPubSub.Client
	logger pubsub.Logger
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

	g.logger.Debugf("published google message %v on topic %v", string(message), topic)

	return nil
}

func (g *googleClient) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	var m = pubsub.NewMessage(ctx)

	t, err := g.getTopic(ctx, topic)
	if err != nil {
		return nil, err
	}

	subscription, err := g.getSubscription(ctx, t)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	err = subscription.Receive(ctx, func(_ context.Context, msg *gcPubSub.Message) {
		defer cancel()

		m = &pubsub.Message{
			Topic:    topic,
			Value:    msg.Data,
			MetaData: msg.Attributes,

			Committer: newGoogleMessage(msg),
		}

		msg.Ack()
	})

	if err != nil {
		g.logger.Errorf("Error getting a message: %s", err.Error())

		return nil, err
	}

	g.logger.Debugf("received google message %v on topic %v", string(m.Value), m.Topic)

	return m, nil
}

func (g *googleClient) getTopic(ctx context.Context, topic string) (*gcPubSub.Topic, error) {
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
	subscription := g.client.Subscription(g.SubscriptionName + "-" + topic.String())

	// check if subscription already exists or not
	ok, err := subscription.Exists(context.Background())
	if err != nil {
		g.logger.Error(errSubscriptionExistCheck.Error() + err.Error())

		return nil, err
	}

	// if subscription is not present, create a new
	if !ok {
		subscription, err = g.client.CreateSubscription(ctx, g.SubscriptionName+"-"+topic.String(), gcPubSub.SubscriptionConfig{
			Topic: topic,
		})

		if err != nil {
			return nil, err
		}
	}

	return subscription, nil
}

type googleMessage struct {
	msg *gcPubSub.Message
}

func newGoogleMessage(msg *gcPubSub.Message) *googleMessage {
	return &googleMessage{msg: msg}
}

func (gmsg *googleMessage) Commit() {
	gmsg.msg.Ack()
}
