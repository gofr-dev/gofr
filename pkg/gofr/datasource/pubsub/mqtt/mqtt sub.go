package mqtt

import (
	"context"
	"strconv"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

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
	if ok {
		return &subs, nil
	}

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
	}

	return &subs, nil
}

func (m *MQTT) createMqttHandler(ctx context.Context, topic string, msgs chan *pubsub.Message) mqtt.MessageHandler {
	return func(_ mqtt.Client, msg mqtt.Message) {
		ctx := context.WithoutCancel(ctx)
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
