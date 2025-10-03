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

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// parseQueryArgs extracts collectTimeout and messageLimit from variadic arguments.
// This can be a package-level function as it doesn't depend on *MQTT state.
func parseQueryArgs(args ...any) (collectTimeout time.Duration, messageLimit int) {
	collectTimeout = defaultQueryCollectTimeout
	messageLimit = defaultQueryMessageLimit

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
		// if the handler's parent context (original Query ctx) is canceled while the message is in flight.
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
func (m *MQTT) collectMessages(queryCtx context.Context, msgChan <-chan *pubsub.Message,
	messageLimit int, topicName string) (*bytes.Buffer, int, error) {
	var resultBuffer bytes.Buffer

	messagesCollected := 0

	for {
		// Early return if limit reached
		if messageLimit > 0 && messagesCollected >= messageLimit {
			return &resultBuffer, messagesCollected, nil
		}

		select {
		case msg, ok := <-msgChan:
			if !ok {
				m.logger.Debugf("Query: msgChan closed unexpectedly while collecting for topic %s", topicName)
				return &resultBuffer, messagesCollected, nil
			}

			m.addMessageToBuffer(&resultBuffer, msg)

			messagesCollected++

		case <-queryCtx.Done():
			return m.handleContextDone(queryCtx, topicName, &resultBuffer, messagesCollected)
		}
	}
}

func (*MQTT) addMessageToBuffer(buffer *bytes.Buffer, msg *pubsub.Message) {
	if buffer.Len() > 0 {
		buffer.WriteByte('\n')
	}

	buffer.Write(msg.Value)
}

func (*MQTT) handleContextDone(queryCtx context.Context, topicName string, buffer *bytes.Buffer,
	collected int) (*bytes.Buffer, int, error) {
	if !errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
		err := fmt.Errorf("%w for topic '%s': %w", errQueryCancelled, topicName, queryCtx.Err())

		return buffer, collected, err
	}

	return buffer, collected, nil
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
