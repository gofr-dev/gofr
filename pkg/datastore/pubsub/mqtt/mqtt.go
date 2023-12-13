package mqtt

import (
	"fmt"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTT struct{
	Client mqtt.Client
	Topic string
	QoS byte
	logger log.Logger
}

type Config struct {
	Hostname string
	Port int
	Username string
	Password string
	ClientID string
	Topic string
	QoS byte
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
	options.AddBroker(fmt.Sprintf("tls://%s:%d", config.Hostname, config.Port))
	options.SetClientID(config.ClientID)
	options.SetUsername(config.Username)
	options.SetPassword(config.Password)
	options.OnConnect = connectHandler
	options.OnConnectionLost = connectLostHandler

	// create the client using the options above
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
	return MQTT{},token.Error()
	}

	return MQTT{},nil
}

func onMessageReceived(client mqtt.Client, message mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", message.Payload(), message.Topic())
}

func (m MQTT) PublishEventWithOptions(key string, value interface{}, headers map[string]string, options *pubsub.PublishOptions) error {
	return errors.Error("Method Not implemented instead use PublishEvent")
}

func (m MQTT) PublishEvent(s string, i interface{}, mp map[string]string) error {
	token := m.Client.Publish(m.Topic, m.QoS, false, s)
	token.Wait()
	// Check for errors during publishing (More on error reporting https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang#readme-error-handling)
	if token.Error() != nil {
		m.logger.Info("Failed to publish to topic")

		return token.Error()
	}

	return nil
}

func (m MQTT) Subscribe() (*pubsub.Message, error) {
	token := m.Client.Subscribe(m.Topic,m.QoS,onMessageReceived)

	if token.Wait() && token.Error() != nil {
		return nil,token.Error()
	}

	return nil,token.Error()
}

func (m MQTT) SubscribeWithCommit(commitFunc pubsub.CommitFunc) (*pubsub.Message, error) {
	return nil,errors.Error("Method Not implemented instead use Subscribe")
}

func (m MQTT) Bind(message []byte, target interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (m MQTT) CommitOffset(offsets pubsub.TopicPartition) {
	m.logger.Info("Method CommitOffset not implmented for MQTT")
}

func (m MQTT) Ping() error {
	err := m.Client.Connect().Error()

	if err  != nil{
		return err
	}

	return nil
}

func (m MQTT) HealthCheck() types.Health {
	//TODO implement me
	panic("implement me")
}

func (m MQTT) IsSet() bool {
	//TODO implement me
	panic("implement me")
}