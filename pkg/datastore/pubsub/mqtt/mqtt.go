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

// MQTT is the struct that implements PublisherSubscriber interface to
// provide functionality for the MQTT as a pubsub
type MQTT struct {
	// contains filtered or unexported fields
	client   mqtt.Client
	logger   log.Logger
	config   *Config
	messages chan *pubsub.Message
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
	RetrieveRetained        bool
	ConnectionRetryDuration int
}

// New establishes a connection to MQTT Broker using the configs and return pubsub.MqttPublisherSubscriber
// with more MQTT focused functionalities related to subscribing(push), unsubscribing and disconnecting from broker
func New(config *Config, logger log.Logger) (pubsub.MQTTPublisherSubscriber, error) {
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
	options.SetResumeSubs(config.RetrieveRetained)

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

	msg := make(chan *pubsub.Message)

	logger.Debugf("connected to MQTT, HostName : %v, Port : %v", config.Hostname, config.Port)

	return &MQTT{config: config, logger: logger, client: client, messages: msg}, nil
}

// Subscribe SubscribeBroker with a subscribing function, called whenever broker publishes a message
func (m *MQTT) Subscribe(subscribeFunc pubsub.SubscribeFunc) error {
	handler := func(_ mqtt.Client, msg mqtt.Message) {
		// for every subscribe increment metric count
		pubsub.SubscribeReceiveCount(msg.Topic(), "")

		pubsubMsg := &pubsub.Message{
			Topic: msg.Topic(),
			Value: string(msg.Payload()),
			Headers: map[string]string{
				"qos":       string(msg.Qos()),
				"retained":  strconv.FormatBool(msg.Retained()),
				"messageID": strconv.Itoa(int(msg.MessageID())),
			},
		}

		// call the user defined function
		err := subscribeFunc(pubsubMsg)
		if err != nil {
			// increment failure count for failed subscribing
			pubsub.SubscribeFailureCount(msg.Topic(), "")
			return
		}

		// increment success counter for successful subscribing
		pubsub.SubscribeSuccessCount(msg.Topic(), "")
	}

	token := m.client.Subscribe(m.config.Topic, m.config.QoS, handler)

	if token.Wait() && token.Error() != nil {
		// increment failure count for failed subscribing
		pubsub.SubscribeFailureCount(m.config.Topic, "")
		return token.Error()
	}

	return nil
}

func (m *MQTT) Publish(payload []byte) error {
	// for every publishing of event
	pubsub.PublishTotalCount(m.config.Topic, "")

	if m.client == nil {
		m.logger.Debug("client not configured")

		// for unsuccessful publish
		pubsub.PublishFailureCount(m.config.Topic, "")

		return errors.Error("client not configured")
	}

	token := m.client.Publish(m.config.Topic, m.config.QoS, m.config.RetrieveRetained, payload)

	// Check for errors during publishing (More on error reporting
	// https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang#readme-error-handling)
	if token.Wait() && token.Error() != nil {
		m.logger.Errorf("error while publishing message, err : %v", token.Error())
		// for unsuccessful publish
		pubsub.PublishFailureCount(m.config.Topic, "")

		return token.Error()
	}

	// for successful publishing
	pubsub.PublishSuccessCount(m.config.Topic, "")

	return nil
}

func (m *MQTT) Unsubscribe() error {
	token := m.client.Unsubscribe(m.config.Topic)
	token.Wait()

	if token.Error() != nil {
		m.logger.Errorf("error while unsubscribing from  topic %s, err : %v", m.config.Topic, token.Error())

		return token.Error()
	}

	return nil
}

func (m *MQTT) Disconnect(waitTime uint) {
	m.client.Disconnect(waitTime)
}

func (m *MQTT) Bind(message []byte, target interface{}) error {
	return json.Unmarshal(message, target)
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
