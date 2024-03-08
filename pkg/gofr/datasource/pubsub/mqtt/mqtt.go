package mqtt

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const messageBuffer = 10

var (
	errProtocolNotProvided = errors.New("protocol not provided")
	errHostNotProvided     = errors.New("hostname not provided")
	errInvalidPort         = errors.New("invalid port")
	errClientNotConfigured = errors.New("client not configured")
)

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
	err := validateConfigs(config)
	if err != nil {
		logger.Errorf("could not initialize MQTT, err : %v", err)

		return nil
	}

	options := getMQTTClientOptions(config, logger)

	// create the client using the options above
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("cannot connect to MQTT, HostName : %v, Port : %v, error : %v", config.Hostname, config.Port, token.Error())

		return &MQTT{config: config, logger: logger}
	}

	msg := make(map[string]chan *pubsub.Message)

	logger.Debugf("connected to MQTT, HostName : %v, Port : %v", config.Hostname, config.Port)

	return &MQTT{Client: client, config: config, logger: logger, msgChanMap: msg, mu: new(sync.RWMutex), metrics: metrics}
}

func getMQTTClientOptions(config *Config, logger Logger) *mqtt.ClientOptions {
	options := mqtt.NewClientOptions()
	options.AddBroker(config.Protocol + "://" + config.Hostname + ":" + strconv.Itoa(config.Port))
	options.SetClientID(config.ClientID)

	if config.ClientID == "" {
		logger.Warnf("client id not provided, please provide a clientID to prevent unexpected behaviors")

		options.SetClientID("gofr_mqtt_client")
	}

	if config.Username != "" {
		options.SetUsername(config.Username)
	}

	if config.Password != "" {
		options.SetPassword(config.Password)
	}

	options.SetOrderMatters(config.Order)
	options.SetResumeSubs(config.RetrieveRetained)

	// upon connection to the client, this is called
	options.OnConnect = func(client mqtt.Client) {
		logger.Debug("Connected")
	}

	// this is called when the connection to the client is lost; it prints "Connection lost" and the corresponding error
	options.OnConnectionLost = func(client mqtt.Client, err error) {
		logger.Errorf("Connection lost: %v", err)
	}

	return options
}

func validateConfigs(conf *Config) error {
	if conf.Protocol == "" {
		return errProtocolNotProvided
	}

	if conf.Hostname == "" {
		return errHostNotProvided
	}

	if conf.Port == 0 {
		return errInvalidPort
	}

	return nil
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

	if m.Client == nil {
		m.logger.Debug("client not configured")

		return errClientNotConfigured
	}

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
	if m == nil {
		return datasource.Health{
			Details: map[string]interface{}{
				"Name": "MQTT",
			},
			Status: "DOWN",
		}
	}

	res := datasource.Health{

		Status: "DOWN",
		Details: map[string]interface{}{
			"Name": "MQTT",
			"Host": m.config.Hostname,
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

func (m *MQTT) CreateTopic(_ context.Context, _ string) error {
	return nil
}

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

	token := m.Client.Subscribe(topic, m.config.QoS, handler)

	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (m *MQTT) Unsubscribe(topic string) error {
	token := m.Client.Unsubscribe(topic)
	token.Wait()

	if token.Error() != nil {
		m.logger.Errorf("error while unsubscribing from  topic %s, err : %v", topic, token.Error())

		return token.Error()
	}

	return nil
}

func (m *MQTT) Disconnect(waitTime uint) {
	m.Client.Disconnect(waitTime)
}

func (m *MQTT) Ping() error {
	err := m.Client.Connect().Error()

	if err != nil {
		return err
	}

	return nil
}
