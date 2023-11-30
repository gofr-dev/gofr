// Package google provides  methods to work with Google Cloud Pub/Sub enabling the publishing and consumption of messages.
package google

import (
	"context"
	"encoding/json"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	gpubsub "cloud.google.com/go/pubsub"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

// Config stores the configuration parameters required to connect to Google Cloud Pub/Sub.
type Config struct {
	TopicName           string
	ProjectID           string
	SubscriptionDetails *Subscription
	Topic               *gpubsub.Topic
	TimeoutDuration     int
	ConnRetryDuration   int
	Subscription        *gpubsub.Subscription
}

// Subscription defines the name of the Pub/Sub subscription.
type Subscription struct {
	Name string
}

// GCPubSub is a client for interacting with Google Cloud Pub/Sub.
type GCPubSub struct {
	config *Config
	client *gpubsub.Client
	logger log.Logger
}

//nolint:gochecknoglobals // The declared global variable can be accessed across multiple functions
var (
	subscribeReceiveCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "gcpubsub_receive_count",
		Help: "Total number of subscribe operation",
	}, []string{"topic", "consumerGroup"})

	subscribeSuccessCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "gcpubsub_success_count",
		Help: "Total number of successful subscribe operation",
	}, []string{"topic", "consumerGroup"})

	subscribeFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "gcpubsub_failure_count",
		Help: "Total number of failed subscribe operation",
	}, []string{"topic", "consumerGroup"})

	publishSuccessCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "gcpubsub_publish_success_count",
		Help: "Counter for the number of messages successfully published",
	}, []string{"topic", "publisherGroup"})

	publishFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "gcpubsub_publish_failure_count",
		Help: "Counter for the number of failed publish operations",
	}, []string{"topic", "publisherGroup"})

	publishTotalCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "gcpubsub_publish_total_count",
		Help: "Counter for the total number of publish operations",
	}, []string{"topic", "publisherGroup"})
)

// New returns new client
func New(config *Config, logger log.Logger) (*GCPubSub, error) {
	_ = prometheus.Register(subscribeReceiveCount)
	_ = prometheus.Register(subscribeSuccessCount)
	_ = prometheus.Register(subscribeFailureCount)
	_ = prometheus.Register(publishSuccessCount)
	_ = prometheus.Register(publishFailureCount)
	_ = prometheus.Register(publishTotalCount)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(config.TimeoutDuration)*time.Second)

	defer cancel()

	client, err := gpubsub.NewClient(ctx, config.ProjectID)
	if err != nil {
		logger.Errorf("Google PubSub: Error creating new client - %v", err)
		return &GCPubSub{client: client, config: config}, err
	}

	logger.Debug("New client has been created successfully!")

	config.Topic = client.Topic(config.TopicName)
	gcpPubsub := &GCPubSub{client: client, config: config, logger: logger}

	err = createTopicsIfNotExist(ctx, gcpPubsub)
	if err != nil {
		return nil, err
	}

	err = createSubscription(ctx, gcpPubsub)

	if err != nil {
		subscribeFailureCount.WithLabelValues(config.TopicName, "").Inc()
		return nil, err
	}

	return gcpPubsub, nil
}
func createTopicsIfNotExist(ctx context.Context, g *GCPubSub) error {
	exist, err := g.config.Topic.Exists(ctx)
	if err != nil {
		g.logger.Debug("Topic existence check failed:", err)
		return err
	}

	if !exist {
		_, err = g.client.CreateTopic(ctx, g.config.TopicName)
		if err != nil {
			g.logger.Debug("Error in creating a Topic")
			return err
		}
	}

	g.logger.Debug("Topic created successfully")

	return nil
}

func createSubscription(ctx context.Context, g *GCPubSub) error {
	g.config.Subscription = g.client.Subscription(g.config.SubscriptionDetails.Name)

	ok, err := g.config.Subscription.Exists(context.Background())
	if err != nil {
		g.logger.Debugf("Unable to check the existence of subscription: " + err.Error())
		return errors.Error("Unable to check the existence of subscription: " + err.Error())
	}

	if !ok {
		g.config.Subscription, err = g.client.CreateSubscription(ctx, g.config.SubscriptionDetails.Name, gpubsub.SubscriptionConfig{
			Topic: g.config.Topic,
		})

		return err
	}

	g.logger.Debug("Subscription created successfully")

	return nil
}

// PublishEventWithOptions publishes message to google Pub/Sub. Ability to provide additional options described in PublishOptions struct
func (g *GCPubSub) PublishEventWithOptions(_ string, value interface{}, headers map[string]string,
	options *pubsub.PublishOptions) (err error) {
	if options == nil {
		options = &pubsub.PublishOptions{}
	}

	topic := g.client.Topic(g.config.TopicName)

	if options.Timestamp.IsZero() {
		options.Timestamp = time.Now()
	}

	valBytes, err := json.Marshal(value)
	if err != nil {
		publishFailureCount.WithLabelValues(options.Topic, "").Inc()
		g.logger.Debug("Error while marshaling the message data: ", err)

		return err
	}

	msg := &gpubsub.Message{
		Data:       valBytes,
		Attributes: headers,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(g.config.TimeoutDuration)*time.Second)

	defer cancel()

	result := topic.Publish(ctx, msg)

	_, err = result.Get(ctx)
	if err != nil {
		publishFailureCount.WithLabelValues(options.Topic, "").Inc()
		g.logger.Debug("Error while publishing the message: ", err)

		return err
	}

	publishSuccessCount.WithLabelValues(options.Topic, "").Inc()

	g.logger.Debugf("Message has been Published")

	return nil
}

// PublishEvent publishes the event to google Pub/Sub
func (g *GCPubSub) PublishEvent(key string, value interface{}, headers map[string]string) error {
	return g.PublishEventWithOptions(key, value, headers, &pubsub.PublishOptions{Topic: g.config.TopicName, Timestamp: time.Now()})
}

// Subscribe read messages from google Pub/Sub configured
func (g *GCPubSub) Subscribe() (*pubsub.Message, error) {
	subscribeReceiveCount.WithLabelValues(g.config.TopicName, "").Inc()

	var res pubsub.Message

	res.Topic = g.config.TopicName

	ctx, cancel := context.WithCancel(context.Background())

	handler := func(ctx context.Context, m *gpubsub.Message) {
		defer cancel()
		g.logger.Debug("Received message: ", string(m.Data))
		res.Value = string(m.Data)
		m.Ack() // Acknowledge that the message has been consumed
	}

	err := g.config.Subscription.Receive(ctx, handler)
	if err != nil {
		subscribeFailureCount.WithLabelValues(g.config.TopicName, "").Inc()
		g.logger.Debug("Error while receiving message: ", err)

		return nil, err
	}

	subscribeSuccessCount.WithLabelValues(g.config.TopicName, "").Inc()

	g.logger.Debug("Message received successfully.")

	return &res, nil
}

/*
SubscribeWithCommit calls the CommitFunc after subscribing message from googlePubSub and based on the return values decides
whether to commit message and consume another message
*/
func (g *GCPubSub) SubscribeWithCommit(commitFunc pubsub.CommitFunc) (*pubsub.Message, error) {
	subscribeReceiveCount.WithLabelValues(g.config.TopicName, "").Inc()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var res pubsub.Message
	res.Topic = g.config.TopicName

	handler := func(_ context.Context, m *gpubsub.Message) {
		g.processMessage(m, &res, commitFunc, cancel)
	}

	for {
		select {
		case <-ctx.Done():
			return &res, nil // Context canceled or timed out
		default:
			if err := g.config.Subscription.Receive(ctx, handler); err != nil {
				subscribeFailureCount.WithLabelValues(g.config.TopicName, "").Inc()
				g.logger.Debug("Error while receiving message: ", err)

				return nil, err
			}

			subscribeSuccessCount.WithLabelValues(g.config.TopicName, "").Inc()
			g.logger.Debug("Message received successfully.")
		}
	}
}

func (g *GCPubSub) processMessage(m *gpubsub.Message, res *pubsub.Message, commitFunc pubsub.CommitFunc,
	cancelFunc context.CancelFunc) {
	g.logger.Debug("Received message: ", string(m.Data))
	res.Value = string(m.Data)

	// Call the commit function
	isCommit, isContinue := commitFunc(&pubsub.Message{
		Topic: g.config.TopicName,
		Value: string(m.Data),
	})

	if isCommit {
		m.Ack() // Acknowledge that the message has been consumed
	}

	if !isContinue {
		cancelFunc()
	}
}

func (g *GCPubSub) Bind(message []byte, target interface{}) error {
	return json.Unmarshal(message, &target)
}

// CommitOffset In Google Cloud Pub/Sub, there is no direct equivalent to committing offsets as in Apache Kafka.
// Pub/Sub is a fully-managed service where the acknowledgment of messages is handled automatically.
// In Pub/Sub, once a message is received and processed by a subscriber, it is automatically acknowledged by the service.
// There is no need to explicitly commit offsets or track offsets manually like in Kafka.
func (g *GCPubSub) CommitOffset(pubsub.TopicPartition) {}

// Ping checks for the health of google Pub/Sub, returns an error if it is down
func (g *GCPubSub) Ping() error {
	if !g.IsSet() {
		g.logger.Debug("Google Pubsub not initialized")
		return errors.Error("Google Pubsub not initialized")
	}

	topic := g.client.Topic(g.config.TopicName)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(g.config.TimeoutDuration)*time.Second)

	defer cancel()

	exists, err := topic.Exists(ctx)
	if err != nil {
		g.logger.Debug("Error while checking topic existence: ", err)
		return err
	}

	if !exists {
		g.logger.Debug("Topic does not exist: ", g.config.TopicName)

		return errors.Error("Topic does not exists")
	}

	g.logger.Debug("Ping successful.")

	return nil
}

// HealthCheck returns the health of the Pub/Sub
func (g *GCPubSub) HealthCheck() types.Health {
	if !g.IsSet() {
		g.logger.Debug("Google Pubsub not initialized")

		return types.Health{
			Name:   datastore.GooglePubSub,
			Status: pkg.StatusDown,
		}
	}

	resp := types.Health{
		Name:   datastore.GooglePubSub,
		Status: pkg.StatusDown,
		Host:   g.config.TopicName,
	}

	err := g.Ping()

	if err != nil {
		g.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: datastore.GooglePubSub, Err: err})

		return resp
	}

	resp.Status = pkg.StatusUp

	g.logger.Debug("Health check successful.")

	return resp
}

// IsSet checks whether google Pub/Sub is initialized or not
func (g *GCPubSub) IsSet() bool {
	return g.client != nil
}
