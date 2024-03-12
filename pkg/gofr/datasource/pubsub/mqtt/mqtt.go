package mqtt

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

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

	config     *Config
	msgChanMap map[string]chan *pubsub.Message

	mu *sync.RWMutex
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
}

// New establishes a connection to MQTT Broker using the configs and return pubsub.MqttPublisherSubscriber
// with more MQTT focused functionalities related to subscribing(push), unsubscribing and disconnecting from broker.
func New(config *Config, logger Logger, metrics Metrics) *MQTT {
	if config.Hostname == "" {
		return getDefaultClient(config, logger, metrics)
	}

	options := getMQTTClientOptions(config, logger)

	// create the client using the options above
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("cannot connect to MQTT, HostName : %v, Port : %v, error : %v", config.Hostname, config.Port, token.Error())

		return &MQTT{Client: client, config: config, logger: logger}
	}

	msg := make(map[string]chan *pubsub.Message)

	logger.Debugf("connected to MQTT, HostName : %v, Port : %v", config.Hostname, config.Port)

	return &MQTT{Client: client, config: config, logger: logger, msgChanMap: msg, mu: new(sync.RWMutex), metrics: metrics}
}

func getDefaultClient(config *Config, logger Logger, metrics Metrics) *MQTT {
	var (
		host     = publicBroker
		port     = 1883
		clientID = getClientID(config.ClientID)
	)

	logger.Debugf("using %v clientID for this session", clientID)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", host, port))
	opts.SetClientID(clientID)
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("cannot connect to MQTT, HostName : %v, Port : %v, error : %v", host, port, token.Error())

		return &MQTT{Client: client, config: config, logger: logger}
	}

	config.Hostname = host
	config.Port = port
	config.ClientID = clientID

	msg := make(map[string]chan *pubsub.Message)

	logger.Debugf("connected to MQTT, HostName : %v, Port : %v", config.Hostname, config.Port)

	return &MQTT{Client: client, config: config, logger: logger, msgChanMap: msg, mu: new(sync.RWMutex), metrics: metrics}
}

func getMQTTClientOptions(config *Config, logger Logger) *mqtt.ClientOptions {
	options := mqtt.NewClientOptions()
	options.AddBroker(fmt.Sprintf("%s://%s:%d", config.Protocol, config.Hostname, config.Port))

	clientID := getClientID(config.ClientID)
	options.SetClientID(clientID)

	logger.Debugf("using %v clientID for this session", clientID)

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
	m.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic)

	var messg = pubsub.NewMessage(ctx)

	m.mu.Lock()
	// get the message channel for the given topic
	msgChan, ok := m.msgChanMap[topic]
	if !ok {
		msgChan = make(chan *pubsub.Message, messageBuffer)
		m.msgChanMap[topic] = msgChan
	}
	m.mu.Unlock()

	handler := func(_ mqtt.Client, msg mqtt.Message) {
		messg.Topic = msg.Topic()
		messg.Value = msg.Payload()
		messg.MetaData = map[string]string{
			"qos":       string(msg.Qos()),
			"retained":  strconv.FormatBool(msg.Retained()),
			"messageID": strconv.Itoa(int(msg.MessageID())),
		}

		messg.Committer = &message{msg: msg}

		// store the message in the channel
		msgChan <- messg
	}

	token := m.Client.Subscribe(topic, m.config.QoS, handler)

	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	m.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic)

	// blocks if there are no messages in the channel
	return <-msgChan, nil
}

func (m *MQTT) Publish(ctx context.Context, topic string, message []byte) error {
	m.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	token := m.Client.Publish(topic, m.config.QoS, m.config.RetrieveRetained, message)

	// Check for errors during publishing (More on error reporting
	// https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang#readme-error-handling)
	if token.Wait() && token.Error() != nil {
		m.logger.Errorf("error while publishing message, err : %v", token.Error())

		return token.Error()
	}

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
		m.logger.Errorf("unable to create topic - %s, error : %v", topic, token.Error())

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
	token := m.Client.Unsubscribe(topic)
	token.Wait()

	if token.Error() != nil {
		m.logger.Errorf("error while unsubscribing from topic %s, err : %v", topic, token.Error())

		return token.Error()
	}

	return nil
}

func (m *MQTT) Disconnect(waitTime uint) {
	m.Client.Disconnect(waitTime)
}

func (m *MQTT) Ping() error {
	connected := m.Client.IsConnected()

	if !connected {
		return errClientNotConnected
	}

	return nil
}
