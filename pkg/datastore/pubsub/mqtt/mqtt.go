package mqtt

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTT struct {
	Client mqtt.Client
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

// upon connection to the client, this is called
var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

// this is called when the connection to the client is lost, it prints "Connection lost" and the corresponding error
var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connection lost: %v", err)
}

// New establishes connection to Kafka using the config provided in KafkaConfig
func New(config *Config, logger log.Logger) (pubsub.PublisherSubscriber, error) {
	options := mqtt.NewClientOptions()
	options.AddBroker("tcp://" + config.Hostname + ":" + strconv.Itoa(config.Port))
	options.SetClientID(config.ClientID)

	if config.Username != "" {
		options.SetUsername(config.Username)
	}

	if config.Password != "" {
		options.SetPassword(config.Password)
	}

	options.SetOrderMatters(config.Order)
	options.OnConnect = connectHandler
	options.OnConnectionLost = connectLostHandler
	options.ConnectRetryInterval = time.Second * time.Duration(config.ConnectionRetryDuration)

	// create the client using the options above
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("cannot connect to MQTT, HostName : %v, Port : %v, error : %v", config.Topic, config.Port, token.Error())

		return &MQTT{config: config, logger: logger}, token.Error()
	}

	logger.Debugf("connected to MQTT, HostName : %v, Port : %v", config.Topic, config.Port)

	return &MQTT{config: config, logger: logger, Client: client}, nil
}

func (m *MQTT) PublishEvent(key string, value interface{}, mp map[string]string) error {
	token := m.Client.Publish(m.config.Topic, m.config.QoS, false, value)
	token.Wait()

	// Check for errors during publishing (More on error reporting https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang#readme-error-handling)
	if token.Error() != nil {
		m.logger.Debug("Failed to publish to topic")

		return token.Error()
	}

	return nil
}

func (m *MQTT) PublishEventWithOptions(key string, value interface{}, headers map[string]string, options *pubsub.PublishOptions) error {
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

	token := m.Client.Subscribe(m.config.Topic, m.config.QoS, handler)

	if token.Wait() && token.Error() != nil {
		// increment failure count for failed subscribing
		pubsub.SubscribeFailureCount(m.config.Topic, "")
		return nil, token.Error()
	}

	// increment success counter for successful subscribing
	pubsub.PublishSuccessCount(m.config.Topic, "")

	return <-msg, nil
}

func (m *MQTT) SubscribeWithCommit(commitFunc pubsub.CommitFunc) (*pubsub.Message, error) {
	return m.Subscribe()
}

func (m *MQTT) Bind(message []byte, target interface{}) error {
	return json.Unmarshal(message, target)
}

func (m *MQTT) CommitOffset(offsets pubsub.TopicPartition) {
}

func (m *MQTT) Ping() error {
	err := m.Client.Connect().Error()

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

	if m.Client == nil {
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

	if m.Client == nil {
		return false
	}

	return true
}
