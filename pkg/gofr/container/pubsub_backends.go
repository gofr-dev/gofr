//go:build !gofr_nopubsub

// The concrete pub/sub backends.
//
// They live behind a build tag because they are the heaviest thing a GoFr binary
// links: Google Pub/Sub alone brings grpc, the Google auth stack and s2a-go --
// 212 packages and 8.9 MB of binary between the three of them -- and a service
// that publishes to none of them still pays for all of them.
//
// The default build includes this file, so nothing changes unless a user asks
// for it with -tags gofr_nopubsub. Container.PubSub is typed as the pubsub.Client
// INTERFACE, which is what makes the swap possible without touching any public
// type or signature.

package container

import (
	"strconv"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/google"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
	"gofr.dev/pkg/gofr/datasource/pubsub/mqtt"
)

// configTrue is the value a boolean config carries when set. It lives in this
// file rather than beside the other container constants because only this file
// uses it -- an untagged home would leave it unused, and so a lint failure, in a
// build made with -tags gofr_nopubsub.
const configTrue = "true"

// pubsubBackendsLinked reports whether the concrete pub/sub clients are compiled
// into this binary. Tests that assert on a real client consult it and skip when
// they are not, so the suite stays meaningful in both build configurations
// instead of failing in one.
const pubsubBackendsLinked = true

func (c *Container) createMqttPubSub(conf config.Config) pubsub.Client {
	var qos byte

	port, err := strconv.Atoi(conf.GetOrDefault("MQTT_PORT", "0"))
	if err != nil {
		c.Logger.Error("Invalid value for MQTT_PORT, using default: 0")
	}

	order, _ := strconv.ParseBool(conf.GetOrDefault("MQTT_MESSAGE_ORDER", "false"))

	retrieveRetained, _ := strconv.ParseBool(conf.GetOrDefault("MQTT_RETRIEVE_RETAINED", "false"))

	keepAlive, err := time.ParseDuration(conf.Get("MQTT_KEEP_ALIVE"))
	if err != nil {
		keepAlive = 30 * time.Second

		c.Logger.Debug("MQTT_KEEP_ALIVE is not set or invalid, setting it to 30 seconds")
	}

	switch conf.Get("MQTT_QOS") {
	case "1":
		qos = 1
	case "2":
		qos = 2
	default:
		qos = 0
	}

	configs := &mqtt.Config{
		Protocol:         conf.GetOrDefault("MQTT_PROTOCOL", "tcp"), // using tcp as default method to connect to broker
		Hostname:         conf.Get("MQTT_HOST"),
		Port:             port,
		Username:         conf.Get("MQTT_USER"),
		Password:         conf.Get("MQTT_PASSWORD"),
		ClientID:         conf.Get("MQTT_CLIENT_ID_SUFFIX"),
		QoS:              qos,
		Order:            order,
		RetrieveRetained: retrieveRetained,
		KeepAlive:        keepAlive,
		CloseTimeout:     0 * time.Millisecond,
	}

	return mqtt.New(configs, c.Logger, c.metricsManager)
}

func (c *Container) createKafkaPubSub(conf config.Config) {
	if conf.Get("PUBSUB_BROKER") == "" {
		return
	}

	partition, err := strconv.Atoi(conf.GetOrDefault("PARTITION_SIZE", "0"))
	if err != nil {
		c.Logger.Error("Invalid value for PARTITION_SIZE, using default: 0")
	}

	// PUBSUB_OFFSET determines the starting position for message consumption in Kafka.
	// This allows control over whether to read historical messages or only new ones:
	// - Default value -1: Start from the latest offset (only consume new messages after consumer starts)
	// - Value 0: Start from the earliest offset (read all historical messages from the beginning)
	// - Positive value: Start from a specific offset position (useful for resuming from a known point)
	// This is particularly important for scenarios like message replay, recovery from failures,
	// or when you only want to process messages that arrive after the consumer is initialized.
	offSet, err := strconv.Atoi(conf.GetOrDefault("PUBSUB_OFFSET", "-1"))
	if err != nil {
		c.Logger.Error("Invalid value for PUBSUB_OFFSET, using default: -1")
	}

	batchSize, err := strconv.Atoi(conf.GetOrDefault("KAFKA_BATCH_SIZE", strconv.Itoa(kafka.DefaultBatchSize)))
	if err != nil {
		c.Logger.Errorf("Invalid value for KAFKA_BATCH_SIZE, using default: %d", kafka.DefaultBatchSize)
	}

	batchBytes, err := strconv.Atoi(conf.GetOrDefault("KAFKA_BATCH_BYTES", strconv.Itoa(kafka.DefaultBatchBytes)))
	if err != nil {
		c.Logger.Errorf("Invalid value for KAFKA_BATCH_BYTES, using default: %d", kafka.DefaultBatchBytes)
	}

	batchTimeout, err := strconv.Atoi(conf.GetOrDefault("KAFKA_BATCH_TIMEOUT", strconv.Itoa(kafka.DefaultBatchTimeout)))
	if err != nil {
		c.Logger.Errorf("Invalid value for KAFKA_BATCH_TIMEOUT, using default: %d", kafka.DefaultBatchTimeout)
	}

	tlsConf := kafka.TLSConfig{
		CertFile:           conf.Get("KAFKA_TLS_CERT_FILE"),
		KeyFile:            conf.Get("KAFKA_TLS_KEY_FILE"),
		CACertFile:         conf.Get("KAFKA_TLS_CA_CERT_FILE"),
		InsecureSkipVerify: conf.Get("KAFKA_TLS_INSECURE_SKIP_VERIFY") == configTrue,
	}

	pubsubBrokers := strings.Split(conf.Get("PUBSUB_BROKER"), ",")

	c.PubSub = kafka.New(&kafka.Config{
		Brokers:          pubsubBrokers,
		Partition:        partition,
		ConsumerGroupID:  conf.Get("CONSUMER_ID"),
		OffSet:           offSet,
		BatchSize:        batchSize,
		BatchBytes:       batchBytes,
		BatchTimeout:     batchTimeout,
		SecurityProtocol: conf.Get("KAFKA_SECURITY_PROTOCOL"),
		SASLMechanism:    conf.Get("KAFKA_SASL_MECHANISM"),
		SASLUser:         conf.Get("KAFKA_SASL_USERNAME"),
		SASLPassword:     conf.Get("KAFKA_SASL_PASSWORD"),
		TLS:              tlsConf,
	}, c.Logger, c.metricsManager)
}

func (c *Container) createGooglePubSub(conf config.Config) {
	c.PubSub = google.New(google.Config{
		ProjectID:        conf.Get("GOOGLE_PROJECT_ID"),
		SubscriptionName: conf.Get("GOOGLE_SUBSCRIPTION_NAME"),
	}, c.Logger, c.metricsManager)
}
