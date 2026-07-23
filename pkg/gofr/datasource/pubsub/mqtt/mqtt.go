package mqtt

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

const (
	publicBroker               = "broker.emqx.io"
	messageBuffer              = 10
	defaultQueryMessageLimit   = 10
	defaultQueryCollectTimeout = 5 * time.Second
)

var (
	errClientNotConnected  = errors.New("mqtt client not connected")
	errEmptyTopicName      = errors.New("empty topic name")
	errSubscriptionTimeout = errors.New("timed out waiting for MQTT subscription")
	errSubscriptionFailed  = errors.New("failed to subscribe to MQTT topic")
	errQueryCancelled      = errors.New("query canceled")
)

type SubscribeFunc func(*pubsub.Message) error

// MQTT is the struct that implements PublisherSubscriber interface to
// provide functionality for the MQTT as a pubsub.
type MQTT struct {
	cm *autopaho.ConnectionManager

	logger  Logger
	metrics Metrics

	config        *Config
	subscriptions map[string]subscription
	queryHandlers []func(paho.PublishReceived) (bool, error)
	mu            *sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

type Config struct {
	Protocol         string
	Hostname         string
	Port             int
	Username         string
	Password         string
	ClientID         string
	QoS              byte
	Order            bool
	RetrieveRetained bool
	KeepAlive        time.Duration
	CloseTimeout     time.Duration
}

type subscription struct {
	msgs chan *pubsub.Message
}

// New establishes a connection to MQTT Broker using the configs and return pubsub.MqttPublisherSubscriber
// with more MQTT focused functionalities related to subscribing(push), unsubscribing and disconnecting from broker.
func New(config *Config, logger Logger, metrics Metrics) *MQTT {
	if config.Hostname == "" {
		return getDefaultClient(config, logger, metrics)
	}

	subs := make(map[string]subscription)
	mu := new(sync.RWMutex)
	ctx, cancel := context.WithCancel(context.Background())

	m := &MQTT{
		config:        config,
		logger:        logger,
		subscriptions: subs,
		mu:            mu,
		metrics:       metrics,
		ctx:           ctx,
		cancel:        cancel,
	}

	logger.Debugf("connecting to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, config.ClientID)

	var urls []*url.URL
	serverURL, err := url.Parse(config.Protocol + "://" + config.Hostname + ":" + strconv.Itoa(config.Port))
	if err != nil {
		logger.Errorf("invalid MQTT URL: %v", err)
	} else {
		urls = append(urls, serverURL)
	}

	cliCfg := autopaho.ClientConfig{
		ServerUrls:                    urls,
		KeepAlive:                     uint16(config.KeepAlive.Seconds()),
		ConnectUsername:               config.Username,
		ConnectPassword:               []byte(config.Password),
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         0,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			logger.Infof("connected to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, config.ClientID)

			// Resubscribe to all topics
			m.mu.RLock()
			defer m.mu.RUnlock()

			for topic := range m.subscriptions {
				_, subErr := cm.Subscribe(context.Background(), &paho.Subscribe{
					Subscriptions: []paho.SubscribeOptions{
						{Topic: topic, QoS: config.QoS},
					},
				})
				if subErr != nil {
					logger.Debugf("failed to resubscribe to topic %s: %v", topic, subErr)
				} else {
					logger.Debugf("resubscribed to topic %s successfully", topic)
				}
			}
		},
		OnConnectError: func(err error) {
			logger.Errorf("could not connect to MQTT at '%v:%v', error: %v", config.Hostname, config.Port, err)
		},
		ClientConfig: paho.ClientConfig{
			ClientID: config.ClientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					// Route to query handlers if any
					m.mu.RLock()
					for _, handler := range m.queryHandlers {
						handled, handlerErr := handler(pr)
						if handled || handlerErr != nil {
							m.mu.RUnlock()
							return handled, handlerErr
						}
					}
					m.mu.RUnlock()

					// Route to normal subscriptions
					return m.handlePublishReceived(pr)
				},
			},
		},
	}

	cm, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		logger.Errorf("could not initialize MQTT connection manager: %v", err)
	}

	m.cm = cm

	// Optional initial wait, autopaho will keep trying in background anyway
	if cm != nil {
		waitCtx, waitCancel := context.WithTimeout(ctx, 2*time.Second)
		defer waitCancel()
		_ = cm.AwaitConnection(waitCtx)
	}

	return m
}

func (m *MQTT) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if m.cm == nil {
		return nil, errClientNotConnected
	}

	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := m.cm.AwaitConnection(waitCtx); err != nil {
		return nil, errClientNotConnected
	}

	m.mu.Lock()

	// get the message channel for the given topic
	subs, ok := m.subscriptions[topic]
	if !ok {
		subs.msgs = make(chan *pubsub.Message, messageBuffer)

		_, err := m.cm.Subscribe(ctx, &paho.Subscribe{
			Subscriptions: []paho.SubscribeOptions{
				{Topic: topic, QoS: m.config.QoS},
			},
		})

		if err != nil {
			m.mu.Unlock()
			m.logger.Errorf("error getting a message from MQTT, error: %v", err)
			return nil, err
		}

		m.subscriptions[topic] = subs
	}

	m.mu.Unlock()

	select {
	// blocks if there are no messages in the channel
	case msg := <-subs.msgs:
		m.metrics.IncrementCounter(msg.Context(), "app_pubsub_subscribe_success_count", "topic", msg.Topic)
		return msg, nil
	case <-ctx.Done():
		return nil, nil
	}
}

// Query retrieves messages from a topic, waiting up to a specified duration and message limit.
func (m *MQTT) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if m.cm == nil {
		return nil, errClientNotConnected
	}
	waitCtx, cancelPing := context.WithTimeout(ctx, 2*time.Second)
	defer cancelPing()
	if err := m.cm.AwaitConnection(waitCtx); err != nil {
		return nil, errClientNotConnected
	}

	if query == "" {
		return nil, errEmptyTopicName
	}

	collectTimeout, messageLimit := parseQueryArgs(args...)

	msgChan := make(chan *pubsub.Message, messageBuffer)
	handler := m.createQueryMessageHandler(ctx, msgChan, query)

	if err := m.subscribeToTopicForQuery(ctx, query, collectTimeout, handler); err != nil {
		return nil, err
	}

	defer func() {
		unsubCtx, cancelUnsub := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelUnsub()
		_, err := m.cm.Unsubscribe(unsubCtx, &paho.Unsubscribe{Topics: []string{query}})
		if err != nil {
			m.logger.Warnf("Query: timed out or error unsubscribing from topic %s: %v", query, err)
		}
	}()

	queryCtx, cancel := context.WithTimeout(ctx, collectTimeout)
	defer cancel()

	resultBuffer, messagesCollected, collectionErr := m.collectMessages(queryCtx, msgChan, messageLimit, query)
	if collectionErr != nil {
		return nil, collectionErr
	}

	if resultBuffer.Len() == 0 && messagesCollected == 0 {
		m.logger.Debugf("Query: no messages collected for topic %s within timeout/limit", query)
	}

	return resultBuffer.Bytes(), nil
}

func (m *MQTT) Publish(ctx context.Context, topic string, message []byte) error {
	if m.cm == nil {
		return errClientNotConnected
	}

	ctx, span, userProps := startPublishSpan(ctx, topic)
	defer span.End()

	m.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	s := time.Now()

	_, err := m.cm.Publish(ctx, &paho.Publish{
		Topic:   topic,
		QoS:     m.config.QoS,
		Retain:  m.config.RetrieveRetained,
		Payload: message,
		Properties: &paho.PublishProperties{
			User: userProps,
		},
	})

	if err != nil {
		m.logger.Errorf("error while publishing message, error: %v", err)
		return err
	}

	t := time.Since(s)

	m.logger.Debug(&pubsub.Log{
		Mode:          "PUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(message),
		Topic:         topic,
		Host:          m.config.Hostname,
		PubSubBackend: backendMQTT,
		Time:          t.Microseconds(),
	})

	m.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

func (m *MQTT) Health() datasource.Health {
	res := datasource.Health{
		Status: "DOWN",
		Details: map[string]any{
			"backend": "MQTT",
			"host":    m.config.Hostname,
		},
	}

	if m.cm == nil {
		m.logger.Errorf("%v", "datasource not initialized")
		return res
	}

	err := m.Ping()
	if err != nil {
		m.logger.Errorf("%v", "health check failed")
		return res
	}

	res.Status = "UP"

	return res
}

func (m *MQTT) CreateTopic(ctx context.Context, topic string) error {
	if m.cm == nil {
		return errClientNotConnected
	}

	_, err := m.cm.Publish(ctx, &paho.Publish{
		Topic:   topic,
		QoS:     m.config.QoS,
		Retain:  false,
		Payload: []byte("topic creation"),
	})

	if err != nil {
		m.logger.Errorf("unable to create topic '%s', error: %v", topic, err)
		return err
	}

	return nil
}

// DeleteTopic is implemented to adhere to the PubSub Client interface
// Note: there is no concept of deletion.
func (*MQTT) DeleteTopic(_ context.Context, _ string) error {
	return nil
}

// Extended Functionalities for MQTT

// SubscribeWithFunction subscribe with a subscribing function, called whenever broker publishes a message.
func (m *MQTT) SubscribeWithFunction(topic string, subscribeFunc SubscribeFunc) error {
	if m.cm == nil {
		return errClientNotConnected
	}

	// We can't easily append to OnPublishReceived dynamically after connection is established with autopaho
	// if we don't have a slice that we synchronize. However, in our ClientConfig, we route to queryHandlers
	// and subscriptions. For SubscribeWithFunction we should either map it to normal subscriptions or handle it explicitly.
	// Since GoFr currently just uses token.Subscribe with a specific handler, we'll append to queryHandlers for now,
	// or create a specific map for functional subscriptions. Let's append to queryHandlers.

	handler := getHandler(subscribeFunc)

	m.mu.Lock()
	m.queryHandlers = append(m.queryHandlers, func(pr paho.PublishReceived) (bool, error) {
		if pr.Packet.Topic == topic || topicMatch(topic, pr.Packet.Topic) {
			return handler(pr)
		}
		return false, nil
	})
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()

	_, err := m.cm.Subscribe(ctx, &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{Topic: topic, QoS: 1}, // default was 1 in previous implementation
		},
	})

	if err != nil {
		return err
	}

	return nil
}
