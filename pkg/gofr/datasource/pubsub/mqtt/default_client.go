package mqtt

import (
	"context"
	"net/url"
	"strconv"
	"sync"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
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
		host = "broker.hivemq.com"
	}

	config.Hostname = host
	config.Port = port
	config.ClientID = clientID

	subs := make(map[string]subscription)
	mu := new(sync.RWMutex)
	ctx, cancel := context.WithCancel(context.Background())

	m := &MQTT{
		config:        config,
		logger:        logger,
		subscriptions: subs,
		mu:            mu,
		metrics:       metrics,
		ctx:           ctx,
		cancel:        cancel,
	}

	logger.Debugf("connecting to default MQTT at '%v:%v' with clientID '%v'", host, port, clientID)

	var urls []*url.URL
	serverURL, err := url.Parse("tcp://" + host + ":" + strconv.Itoa(port))
	if err != nil {
		logger.Errorf("invalid MQTT URL: %v", err)
	} else {
		urls = append(urls, serverURL)
	}

	cliCfg := autopaho.ClientConfig{
		ServerUrls:                    urls,
		KeepAlive:                     uint16(config.KeepAlive.Seconds()),
		ConnectUsername:               config.Username,
		ConnectPassword:               []byte(config.Password),
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         0,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			logger.Infof("connected to MQTT at '%v:%v' with clientID '%v'", config.Hostname, config.Port, config.ClientID)

			// Resubscribe to all topics
			m.mu.RLock()
			defer m.mu.RUnlock()

			for topic := range m.subscriptions {
				_, subErr := cm.Subscribe(context.Background(), &paho.Subscribe{
					Subscriptions: []paho.SubscribeOptions{
						{Topic: topic, QoS: config.QoS},
					},
				})
				if subErr != nil {
					logger.Debugf("failed to resubscribe to topic %s: %v", topic, subErr)
				} else {
					logger.Debugf("resubscribed to topic %s successfully", topic)
				}
			}
		},
		OnConnectError: func(err error) {
			logger.Errorf("could not connect to MQTT at '%v:%v', error: %v", config.Hostname, config.Port, err)
		},
		ClientConfig: paho.ClientConfig{
			ClientID: config.ClientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					// Route to query handlers if any
					m.mu.RLock()
					for _, handler := range m.queryHandlers {
						handled, handlerErr := handler(pr)
						if handled || handlerErr != nil {
							m.mu.RUnlock()
							return handled, handlerErr
						}
					}
					m.mu.RUnlock()

					// Route to normal subscriptions
					return m.handlePublishReceived(pr)
				},
			},
		},
	}

	cm, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		logger.Errorf("could not initialize MQTT connection manager: %v", err)
	}

	m.cm = cm

	return m
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
