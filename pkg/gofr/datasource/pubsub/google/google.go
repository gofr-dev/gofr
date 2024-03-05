package google

import (
	"context"
	"errors"
	"gofr.dev/pkg/gofr/datasource"
	"google.golang.org/api/iterator"
	"time"

	gcPubSub "cloud.google.com/go/pubsub"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

var (
	errProjectIDNotProvided    = errors.New("google project id not provided")
	errSubscriptionNotProvided = errors.New("subscription name not provided")
)

type Config struct {
	ProjectID        string
	SubscriptionName string
}

type googleClient struct {
	Config

	client  *gcPubSub.Client
	logger  pubsub.Logger
	metrics Metrics
}

func (g *googleClient) Health() (health datasource.Health) {
	health.Details = make(map[string]interface{})

	health.Status = "UP"
	health.Details["backend"] = "GOOGLE"

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	it := g.client.Topics(ctx)

	topics := make(map[string]interface{})

	for {
		topic, err := it.Next()
		if err == iterator.Done {
			break
		}

		if err != nil {
			health.Status = "DOWN"
		}

		if topic != nil {
			topics[topic.ID()] = topic
		}
	}

	health.Details["writers"] = topics

	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	subIt := g.client.Subscriptions(ctx)

	subscriptions := make(map[string]interface{})

	for {
		subcription, err := subIt.Next()
		if err == iterator.Done {
			break
		}

		if err != nil {
			health.Status = "DOWN"
		}

		if subcription != nil {
			subscriptions[subcription.ID()] = subcription
		}
	}

	health.Details["readers"] = subscriptions

	return health
}

//nolint:revive // We do not want anyone using the client without initialization steps.
func New(conf Config, logger pubsub.Logger, metrics Metrics) *googleClient {
	err := validateConfigs(&conf)
	if err != nil {
		logger.Errorf("google pubsub could not be configured, err : %v", err)

		return nil
	}

	client, err := gcPubSub.NewClient(context.Background(), conf.ProjectID)
	if err != nil {
		return &googleClient{
			Config: conf,
		}
	}

	return &googleClient{
		Config:  conf,
		client:  client,
		logger:  logger,
		metrics: metrics,
	}
}

func validateConfigs(conf *Config) error {
	if conf.ProjectID == "" {
		return errProjectIDNotProvided
	}

	if conf.SubscriptionName == "" {
		return errSubscriptionNotProvided
	}

	return nil
}

func (g *googleClient) Publish(ctx context.Context, topic string, message []byte) error {
	g.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	t, err := g.getTopic(ctx, topic)
	if err != nil {
		return err
	}

	result := t.Publish(ctx, &gcPubSub.Message{
		Data:        message,
		PublishTime: time.Now(),
	})

	_, err = result.Get(ctx)
	if err != nil {
		return err
	}

	g.logger.Debugf("published google message %v on topic %v", string(message), topic)

	g.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

func (g *googleClient) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	g.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic)

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
	})

	if err != nil {
		g.logger.Errorf("Error getting a message: %s", err.Error())

		return nil, err
	}

	g.logger.Debugf("received google message %v on topic %v", string(m.Value), m.Topic)

	g.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic)

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
	subscription := g.client.Subscription(g.SubscriptionName + "-" + topic.ID())

	// check if subscription already exists or not
	ok, err := subscription.Exists(context.Background())
	if err != nil {
		g.logger.Errorf("unable to check the existence of subscription, err : %v ", err.Error())

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
