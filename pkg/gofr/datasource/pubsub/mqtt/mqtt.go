package mqtt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	publicBroker               = "broker.emqx.io"
	messageBuffer              = 10
	defaultRetryTimeout        = 10 * time.Second
	maxRetryTimeout            = 1 * time.Minute
	defaultQueryMessageLimit   = 10
	defaultQueryCollectTimeout = 5 * time.Second
	unsubscribeOpTimeout       = 2 * time.Second
)

var (
	errClientNotConnected  = errors.New("mqtt client not connected")
	errEmptyTopicName      = errors.New("empty topic name")
	errSubscriptionTimeout = errors.New("timed out waiting for MQTT subscription")
	errSubscriptionFailed  = errors.New("failed to subscribe to MQTT topic")
	errQueryCancelled      = errors.New("query cancelled")
)

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

// Query retrieves messages from a topic, waiting up to a specified duration and message limit.
func (m *MQTT) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if !m.Client.IsConnected() {
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
		unsubToken := m.Client.Unsubscribe(query)
		if !unsubToken.WaitTimeout(unsubscribeOpTimeout) {
			m.logger.Warnf("Query: timed out unsubscribing from topic %s", query)
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

// parseQueryArgs extracts collectTimeout and messageLimit from variadic arguments.
// This can be a package-level function as it doesn't depend on *MQTT state.
func parseQueryArgs(args ...any) (time.Duration, int) {
	collectTimeout := defaultQueryCollectTimeout
	messageLimit := defaultQueryMessageLimit

	if len(args) > 0 {
		if val, ok := args[0].(time.Duration); ok {
			collectTimeout = val
		}
	}

	if len(args) > 1 {
		if val, ok := args[1].(int); ok {
			messageLimit = val
		}
	}

	return collectTimeout, messageLimit
}

// createQueryMessageHandler creates the MQTT message handler for the Query method.
func (m *MQTT) createQueryMessageHandler(ctx context.Context, msgChan chan<- *pubsub.Message, topicForLogging string) mqtt.MessageHandler {
	return func(_ mqtt.Client, msg mqtt.Message) {
		// Use context.WithoutCancel to ensure the message processing isn't prematurely stopped
		// if the handler's parent context (original Query ctx) is cancelled while the message is in flight.
		messageCtx := context.WithoutCancel(ctx)
		message := pubsub.NewMessage(messageCtx)

		message.Topic = msg.Topic()
		message.Value = msg.Payload()
		message.MetaData = map[string]string{
			"qos":       string(msg.Qos()),
			"retained":  strconv.FormatBool(msg.Retained()),
			"messageID": strconv.Itoa(int(msg.MessageID())),
		}

		select {
		case msgChan <- message:
		default:
			m.logger.Debugf("Query: msgChan full for topic %s, message dropped during collection", topicForLogging)
		}
	}
}

// subscribeToTopicForQuery handles the MQTT subscription logic for the Query method.
func (m *MQTT) subscribeToTopicForQuery(ctx context.Context, topicName string, timeout time.Duration, handler mqtt.MessageHandler) error {
	token := m.Client.Subscribe(topicName, m.config.QoS, handler)

	if !token.WaitTimeout(timeout) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("context error during MQTT subscription to '%s': %w", topicName, ctxErr)
		}

		// If token has an error, it means WaitTimeout likely hit its own timeout AND there was an underlying subscription error.
		if tokenErr := token.Error(); tokenErr != nil {
			return fmt.Errorf("%w to topic '%s' (timed out with underlying error): %w", errSubscriptionFailed, topicName, tokenErr)
		}

		// Fallback: WaitTimeout returned false, context is fine, token.Error() is nil. This is the direct timeout.
		return fmt.Errorf("%w for topic '%s'", errSubscriptionTimeout, topicName)
	}

	if tokenErr := token.Error(); tokenErr != nil {
		return fmt.Errorf("%w to '%s': %w", errSubscriptionFailed, topicName, tokenErr)
	}

	return nil
}

// collectMessages handles the message collection loop for the Query method.
func (m *MQTT) collectMessages(queryCtx context.Context, msgChan <-chan *pubsub.Message, messageLimit int, topicName string) (*bytes.Buffer, int, error) {
	var resultBuffer bytes.Buffer

	messagesCollected := 0

loop:
	for {
		if messageLimit > 0 && messagesCollected >= messageLimit {
			break loop
		}

		select {
		case msg, ok := <-msgChan:
			if !ok {
				m.logger.Debugf("Query: msgChan closed unexpectedly while collecting for topic %s", topicName)
				break loop
			}

			if resultBuffer.Len() > 0 {
				resultBuffer.WriteByte('\n')
			}

			resultBuffer.Write(msg.Value)
			messagesCollected++
		case <-queryCtx.Done():
			if !errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
				return &resultBuffer, messagesCollected, fmt.Errorf("%w for topic '%s': %w", errQueryCancelled, topicName, queryCtx.Err())
			}

			break loop
		}
	}

	return &resultBuffer, messagesCollected, nil
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

	token := m.Client.Publish(topic, m.config.QoS, true, message)

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
	token := m.Client.Publish(topic, m.config.QoS, false, []byte("topic creation"))
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
