package mqtt

import (
	"context"
	"net/url"
	"strconv"
	"sync"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/google/uuid"
)

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

	cliCfg := getClientConfig(config, logger, m, urls)

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
