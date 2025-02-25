package mqtt

import (
	"fmt"
	"math"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

const backoffMultiplier = 2

func getDefaultClient(config *Config, logger Logger, metrics Metrics) *MQTT {
	var (
		host     = publicBroker
		port     = 1883
		clientID = getClientID(config.ClientID)
	)

	if config.Username == "gofr-mqtt-test" {
		host = "test.mosquitto.org"
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", host, port))
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(config.KeepAlive)

	subscriptions := make(map[string]subscription)
	mu := new(sync.RWMutex)

	opts.SetOnConnectHandler(createReconnectHandler(mu, config, subscriptions, logger))
	opts.SetConnectionLostHandler(createConnectionLostHandler(logger))
	opts.SetReconnectingHandler(createReconnectingHandler(logger, config))

	client := mqtt.NewClient(opts)

	mqttClient := &MQTT{
		Client:        client,
		config:        config,
		logger:        logger,
		subscriptions: subscriptions,
		mu:            mu,
		metrics:       metrics,
	}

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("could not connect to MQTT at '%v:%v', error: %v", config.Hostname, config.Port, token.Error())

		go retryDefaultConnect(client, config, logger, opts)

		return mqttClient
	}

	config.Hostname = host
	config.Port = port
	config.ClientID = clientID

	msg := make(map[string]subscription)

	logger.Infof("connected to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, clientID)

	return &MQTT{Client: client, config: config, logger: logger, subscriptions: msg, mu: new(sync.RWMutex), metrics: metrics}
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
	options.SetAutoReconnect(true)
	options.SetKeepAlive(config.KeepAlive)

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

func retryDefaultConnect(client mqtt.Client, config *Config, logger Logger, options *mqtt.ClientOptions) {
	backoff := defaultRetryTimeout

	for {
		token := client.Connect()
		if token.Wait() && token.Error() == nil {
			logger.Infof("connected to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, options.ClientID)

			return
		}

		logger.Errorf("could not connect to MQTT at '%v:%v', error: %v", config.Hostname, config.Port, token.Error())
		time.Sleep(backoff)
		backoff = time.Duration(math.Min(float64(backoff*backoffMultiplier), float64(maxRetryTimeout)))
	}
}
