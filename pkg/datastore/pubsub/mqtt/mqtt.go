package mqtt

import (
	"encoding/json"
	"strconv"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

type MQTT struct {
	client mqtt.Client
	logger log.Logger
	config *Config
}

type Config struct {
	Protocol                string
	Hostname                string
	Port                    int
	Username                string
	Password                string
	ClientID                string
	Topic                   string
	QoS                     byte
	Order                   bool
	ConnectionRetryDuration int
}

// New establishes connection to Kafka using the config provided in KafkaConfig
func New(config *Config, logger log.Logger) (pubsub.PublisherSubscriber, error) {
	pubsub.RegisterMetrics()

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

	// upon connection to the client, this is called
	options.OnConnect = func(client mqtt.Client) {
		logger.Debug("Connected")
	}

	// this is called when the connection to the client is lost; it prints "Connection lost" and the corresponding error
	options.OnConnectionLost = func(client mqtt.Client, err error) {
		logger.Errorf("Connection lost: %v", err)
	}

	// create the client using the options above
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("cannot connect to MQTT, HostName : %v, Port : %v, error : %v", config.Hostname, config.Port, token.Error())

		return &MQTT{config: config, logger: logger}, token.Error()
	}

	logger.Debugf("connected to MQTT, HostName : %v, Port : %v", config.Hostname, config.Port)

	return &MQTT{config: config, logger: logger, client: client}, nil
}

func (m *MQTT) PublishEvent(_ string, value interface{}, _ map[string]string) error {
	// for every publishing of event
	pubsub.PublishTotalCount(m.config.Topic, "")

	if m.client == nil {
		m.logger.Debug("client not configured")
		// for unsuccessful publish
		pubsub.PublishFailureCount(m.config.Topic, "")

		return errors.Error("client not configured")
	}

	// marshal the value to convert it to bytes
	value, _ = json.Marshal(value)

	token := m.client.Publish(m.config.Topic, m.config.QoS, false, value)
	token.Wait()

	// Check for errors during publishing (More on error reporting
	// https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang#readme-error-handling)
	if token.Error() != nil {
		m.logger.Debug("Failed to publish to topic")
		// for unsuccessful publish
		pubsub.PublishFailureCount(m.config.Topic, "")

		return token.Error()
	}

	// for successful publishing
	pubsub.PublishSuccessCount(m.config.Topic, "")

	return nil
}

func (m *MQTT) PublishEventWithOptions(key string, value interface{}, headers map[string]string, _ *pubsub.PublishOptions) error {
	return m.PublishEvent(key, value, headers)
}

func (m *MQTT) Subscribe() (*pubsub.Message, error) {
	// for every subscribe increment metric count
	pubsub.SubscribeReceiveCount(m.config.Topic, "")

	msg := make(chan *pubsub.Message)

	handler := func(_ mqtt.Client, message mqtt.Message) {
		msg <- &pubsub.Message{
			Value: string(message.Payload()),
			Topic: message.Topic(),
		}
	}

	token := m.client.Subscribe(m.config.Topic, m.config.QoS, handler)

	if token.Wait() && token.Error() != nil {
		// increment failure count for failed subscribing
		pubsub.SubscribeFailureCount(m.config.Topic, "")
		return nil, token.Error()
	}

	// increment success counter for successful subscribing
	pubsub.SubscribeSuccessCount(m.config.Topic, "")

	return <-msg, nil
}

func (m *MQTT) SubscribeWithCommit(_ pubsub.CommitFunc) (*pubsub.Message, error) {
	return m.Subscribe()
}

func (m *MQTT) Bind(message []byte, target interface{}) error {
	return json.Unmarshal(message, target)
}

func (m *MQTT) CommitOffset(_ pubsub.TopicPartition) {
}

func (m *MQTT) Ping() error {
	err := m.client.Connect().Error()

	if err != nil {
		return err
	}

	return nil
}

func (m *MQTT) HealthCheck() types.Health {
	if m == nil {
		return types.Health{
			Name:   datastore.Mqtt,
			Status: pkg.StatusDown,
		}
	}

	res := types.Health{
		Name:     datastore.Mqtt,
		Status:   pkg.StatusDown,
		Host:     m.config.Hostname,
		Database: m.config.Topic,
	}

	if m.client == nil {
		m.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: datastore.Mqtt, Reason: "client is not initialized"})
		return res
	}

	err := m.Ping()
	if err != nil {
		m.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: datastore.Mqtt, Err: err})
		return res
	}

	res.Status = pkg.StatusUp

	return res
}

func (m *MQTT) IsSet() bool {
	if m == nil {
		return false
	}

	if m.client == nil {
		return false
	}

	return true
}
