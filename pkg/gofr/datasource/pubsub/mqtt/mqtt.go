package mqtt

import (
	"context"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

const (
	publicBroker        = "broker.emqx.io"
	messageBuffer       = 10
	defaultRetryTimeout = 10 * time.Second
	maxRetryTimeout     = 1 * time.Minute
)

var errClientNotConnected = errors.New("mqtt client not connected")

type SubscribeFunc func(*pubsub.Message) error

// MQTT is the struct that implements PublisherSubscriber interface to
// provide functionality for the MQTT as a pubsub.
type MQTT struct {
	// contains filtered or unexported fields
	mqtt.Client

	logger  Logger
	metrics Metrics

	config        *Config
	subscriptions map[string]subscription
	mu            *sync.RWMutex
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
	msgs    chan *pubsub.Message
	handler func(_ mqtt.Client, msg mqtt.Message)
}

// New establishes a connection to MQTT Broker using the configs and return pubsub.MqttPublisherSubscriber
// with more MQTT focused functionalities related to subscribing(push), unsubscribing and disconnecting from broker.
func New(config *Config, logger Logger, metrics Metrics) *MQTT {
	if config.Hostname == "" {
		return getDefaultClient(config, logger, metrics)
	}

	options := getMQTTClientOptions(config)
	subs := make(map[string]subscription)
	mu := new(sync.RWMutex)

	logger.Debugf("connecting to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, config.ClientID)

	options.SetOnConnectHandler(createReconnectHandler(mu, config, subs, logger))
	options.SetConnectionLostHandler(createConnectionLostHandler(logger))
	options.SetReconnectingHandler(createReconnectingHandler(logger, config))
	// create the client using the options above
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("could not connect to MQTT at '%v:%v', error: %v", config.Hostname, config.Port, token.Error())

		go retryConnect(client, config, logger, options)
	} else {
		logger.Infof("connected to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, options.ClientID)
	}

	return &MQTT{Client: client, config: config, logger: logger, subscriptions: subs, mu: mu, metrics: metrics}
}

func (m *MQTT) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if !m.Client.IsConnected() {
		time.Sleep(defaultRetryTimeout)

		return nil, errClientNotConnected
	}

	m.mu.Lock()

	// get the message channel for the given topic
	subs, ok := m.subscriptions[topic]
	if !ok {
		subs.msgs = make(chan *pubsub.Message, messageBuffer)
		subs.handler = m.createMqttHandler(ctx, topic, subs.msgs)
		token := m.Client.Subscribe(topic, m.config.QoS, subs.handler)

		if token.Wait() && token.Error() != nil {
			m.mu.Unlock()
			m.logger.Errorf("error getting a message from MQTT, error: %v", token.Error())

			return nil, token.Error()
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

func (m *MQTT) createMqttHandler(_ context.Context, topic string, msgs chan *pubsub.Message) mqtt.MessageHandler {
	return func(_ mqtt.Client, msg mqtt.Message) {
		ctx := context.Background()
		ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "mqtt-subscribe")

		defer span.End()

		m.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic)

		var messg = pubsub.NewMessage(context.WithoutCancel(ctx))
		messg.Topic = msg.Topic()
		messg.Value = msg.Payload()
		messg.MetaData = map[string]string{
			"qos":       string(msg.Qos()),
			"retained":  strconv.FormatBool(msg.Retained()),
			"messageID": strconv.Itoa(int(msg.MessageID())),
		}

		messg.Committer = &message{msg: msg}

		// store the message in the channel
		msgs <- messg

		m.logger.Debug(&pubsub.Log{
			Mode:          "SUB",
			CorrelationID: span.SpanContext().TraceID().String(),
			MessageValue:  string(msg.Payload()),
			Topic:         msg.Topic(),
			Host:          m.config.Hostname,
			PubSubBackend: "MQTT",
		})
	}
}
func (m *MQTT) Publish(ctx context.Context, topic string, message []byte) error {
	_, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "mqtt-publish")
	defer span.End()

	m.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	s := time.Now()

	token := m.Client.Publish(topic, m.config.QoS, m.config.RetrieveRetained, message)

	// Check for errors during publishing (More on error reporting
	// https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang#readme-error-handling)
	if token.Wait() && token.Error() != nil {
		m.logger.Errorf("error while publishing message, error: %v", token.Error())

		return token.Error()
	}

	t := time.Since(s)

	m.logger.Debug(&pubsub.Log{
		Mode:          "PUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(message),
		Topic:         topic,
		Host:          m.config.Hostname,
		PubSubBackend: "MQTT",
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

	if m.Client == nil {
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

func (m *MQTT) CreateTopic(_ context.Context, topic string) error {
	token := m.Client.Publish(topic, m.config.QoS, m.config.RetrieveRetained, []byte("topic creation"))
	token.Wait()

	if token.Error() != nil {
		m.logger.Errorf("unable to create topic '%s', error: %v", topic, token.Error())

		return token.Error()
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
	token := m.Client.Subscribe(topic, 1, getHandler(subscribeFunc))

	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func getHandler(subscribeFunc SubscribeFunc) func(client mqtt.Client, msg mqtt.Message) {
	return func(_ mqtt.Client, msg mqtt.Message) {
		pubsubMsg := &pubsub.Message{
			Topic: msg.Topic(),
			Value: msg.Payload(),
			MetaData: map[string]string{
				"qos":       string(msg.Qos()),
				"retained":  strconv.FormatBool(msg.Retained()),
				"messageID": strconv.Itoa(int(msg.MessageID())),
			},
		}

		// call the user defined function
		_ = subscribeFunc(pubsubMsg)
	}
}

func (m *MQTT) Unsubscribe(topic string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token := m.Client.Unsubscribe(topic)
	token.Wait()

	if token.Error() != nil {
		m.logger.Errorf("error while unsubscribing from topic '%s', error: %v", topic, token.Error())

		return token.Error()
	}

	sub, ok := m.subscriptions[topic]
	if ok {
		close(sub.msgs)
		delete(m.subscriptions, topic)
	}

	return nil
}

func (m *MQTT) Close() error {
	timeout := m.config.CloseTimeout

	return m.Disconnect(uint(math.Min(float64(timeout.Milliseconds()), float64(math.MaxUint32))))
}

func (m *MQTT) Disconnect(waitTime uint) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var err error

	for topic := range m.subscriptions {
		unsubscribeErr := m.Unsubscribe(topic)
		if err != nil {
			err = errors.Join(err, unsubscribeErr)

			m.logger.Errorf("Error closing Subscription: %v", err)
		}
	}

	m.Client.Disconnect(waitTime)

	return err
}

func (m *MQTT) Ping() error {
	connected := m.Client.IsConnected()

	if !connected {
		return errClientNotConnected
	}

	return nil
}

func retryConnect(client mqtt.Client, config *Config, logger Logger, options *mqtt.ClientOptions) {
	for {
		token := client.Connect()
		if token.Wait() && token.Error() == nil {
			logger.Infof("connected to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, options.ClientID)

			return
		}

		logger.Errorf("could not connect to MQTT at '%v:%v', error: %v", config.Hostname, config.Port, token.Error())
		time.Sleep(defaultRetryTimeout)
	}
}

func createReconnectHandler(mu *sync.RWMutex, config *Config, subs map[string]subscription,
	logger Logger) mqtt.OnConnectHandler {
	return func(client mqtt.Client) {
		// Re-subscribe to all topics after reconnecting
		mu.RLock()
		defer mu.RUnlock()

		for topic, sub := range subs {
			token := client.Subscribe(topic, config.QoS, sub.handler)
			if token.Wait() && token.Error() != nil {
				logger.Debugf("failed to resubscribe to topic %s: %v", topic, token.Error())
			} else {
				logger.Debugf("resubscribed to topic %s successfully", topic)
			}
		}
	}
}

func createConnectionLostHandler(logger Logger) func(_ mqtt.Client, err error) {
	return func(_ mqtt.Client, err error) {
		logger.Errorf("mqtt connection lost, error: %v", err.Error())
	}
}

func createReconnectingHandler(logger Logger, config *Config) func(mqtt.Client, *mqtt.ClientOptions) {
	return func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		logger.Infof("reconnecting to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, config.ClientID)
	}
}
