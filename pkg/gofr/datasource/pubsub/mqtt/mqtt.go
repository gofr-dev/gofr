package mqtt

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

const (
	publicBroker  = "broker.hivemq.com"
	messageBuffer = 10
)

var errClientNotConnected = errors.New("client not connected")

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
}

type subscription struct {
	msgs    chan *pubsub.Message
	handler func(_ mqtt.Client, msg mqtt.Message)
}

// New establishes a connection to MQTT Broker using the configs and return pubsub.MqttPublisherSubscriber
// with more MQTT focused functionalities related to subscribing(push), unsubscribing and disconnecting from broker.
func New(config *Config, logger Logger, metrics Metrics) *MQTT {
	var options *mqtt.ClientOptions
	if config.Hostname == "" {
		options = getDefaultClientOpts(config)
	} else {
		options = getMQTTClientOptions(config)
	}

	subs := make(map[string]subscription)
	mu := new(sync.RWMutex)

	options.SetOrderMatters(config.Order)
	options.SetResumeSubs(config.RetrieveRetained)
	options.SetAutoReconnect(true)
	options.SetKeepAlive(config.KeepAlive)

	options.SetAutoReconnect(true)
	options.SetKeepAlive(config.KeepAlive)
	options.SetOnConnectHandler(createReconnectHandler(mu, config, subs))
	options.SetConnectionLostHandler(createConnectionLostHandler(logger))
	options.SetReconnectingHandler(createReconnectingHandler(logger, config))

	logger.Debugf("connecting to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, config.ClientID)

	// create the client using the options above
	client := mqtt.NewClient(options)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("could not connect to MQTT at '%v:%v', error: %v", config.Hostname, config.Port, token.Error())
	} else {
		logger.Infof("connected to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, options.ClientID)
	}

	return &MQTT{Client: client, config: config, logger: logger, subscriptions: subs, mu: mu, metrics: metrics}
}

func getDefaultClientOpts(config *Config) *mqtt.ClientOptions {
	var (
		host     = publicBroker
		port     = 1883
		clientID = getClientID(config.ClientID)
	)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", host, port))
	opts.SetClientID(clientID)

	config.Hostname = host
	config.Port = port
	config.ClientID = clientID

	return opts
}

func getMQTTClientOptions(config *Config) *mqtt.ClientOptions {
	options := mqtt.NewClientOptions()
	options.AddBroker(fmt.Sprintf("%s://%s:%d", config.Protocol, config.Hostname, config.Port))

	clientID := getClientID(config.ClientID)
	options.SetClientID(clientID)

	if config.Username != "" {
		options.SetUsername(config.Username)
	}

	if config.Password != "" {
		options.SetPassword(config.Password)
	}

	options.SetOrderMatters(config.Order)
	options.SetResumeSubs(config.RetrieveRetained)

	return options
}

func getClientID(clientID string) string {
	if clientID != "" {
		clientID = "-" + clientID
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return "gofr-mqtt-default-client-id" + clientID
	}

	return id.String() + clientID
}

func (m *MQTT) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	subs, err := m.getSub(ctx, topic)
	if err != nil {
		return nil, err
	}

	if subs == nil {
		return nil, nil
	}
	// blocks if there are no messages in the channel and context not canceled
	select {
	case <-ctx.Done():
		return nil, nil
	case msg := <-subs.msgs:
		m.metrics.IncrementCounter(msg.Context(), "app_pubsub_subscribe_success_count", "topic", msg.Topic)
		return msg, nil
	}
}

func (m *MQTT) getSub(ctx context.Context, topic string) (*subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// get the message channel for the given topic
	subs, ok := m.subscriptions[topic]
	if !ok {
		subs.msgs = make(chan *pubsub.Message, messageBuffer)
		subs.handler = m.createMqttHandler(ctx, topic, subs.msgs)
		token := m.Client.Subscribe(topic, m.config.QoS, subs.handler)
		select {
		case <-token.Done():
			if token.Error() != nil {
				m.logger.Errorf("error getting a message from MQTT, error: %v", token.Error())
				return &subs, token.Error()
			}

			m.subscriptions[topic] = subs
		case <-ctx.Done():
			return &subs, nil
		}
	}

	return &subs, nil
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
		Details: map[string]interface{}{
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
func (m *MQTT) DeleteTopic(_ context.Context, _ string) error {
	return nil
}

// Extended Functionalities for MQTT

// SubscribeWithFunction subscribe with a subscribing function, called whenever broker publishes a message.
func (m *MQTT) SubscribeWithFunction(topic string, subscribeFunc SubscribeFunc) error {
	handler := func(_ mqtt.Client, msg mqtt.Message) {
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
		err := subscribeFunc(pubsubMsg)
		if err != nil {
			return
		}
	}

	token := m.Client.Subscribe(topic, 1, handler)

	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
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

func (m *MQTT) Close(_ context.Context) error {
	const closeTimeoutMs = 250

	m.Client.Disconnect(closeTimeoutMs)

	return nil
}

func (m *MQTT) Disconnect(waitTime uint) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for topic := range m.subscriptions {
		_ = m.Unsubscribe(topic)
	}

	m.Client.Disconnect(waitTime)
}

func (m *MQTT) Ping() error {
	connected := m.Client.IsConnected()

	if !connected {
		return errClientNotConnected
	}

	return nil
}

func createReconnectHandler(mu *sync.RWMutex, config *Config, subs map[string]subscription) func(c mqtt.Client) {
	return func(c mqtt.Client) {
		mu.RLock()
		defer mu.RUnlock()

		for k, v := range subs {
			c.Subscribe(k, config.QoS, v.handler)
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
