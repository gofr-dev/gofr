package kafka

import "errors"

var (
	ErrConsumerGroupNotProvided    = errors.New("consumer group id not provided")
	errFailedToConnectBrokers      = errors.New("failed to connect to any kafka brokers")
	errBrokerNotProvided           = errors.New("kafka broker address not provided")
	errPublisherNotConfigured      = errors.New("can't publish message. Publisher not configured or topic is empty")
	errBatchSize                   = errors.New("KAFKA_BATCH_SIZE must be greater than 0")
	errBatchBytes                  = errors.New("KAFKA_BATCH_BYTES must be greater than 0")
	errBatchTimeout                = errors.New("KAFKA_BATCH_TIMEOUT must be greater than 0")
	errClientNotConnected          = errors.New("kafka client not connected")
	errUnsupportedSASLMechanism    = errors.New("unsupported SASL mechanism")
	errSASLCredentialsMissing      = errors.New("SASL credentials missing")
	errUnsupportedSecurityProtocol = errors.New("unsupported security protocol")
	errNoActiveConnections         = errors.New("no active connections to brokers")
	errCACertFileRead              = errors.New("failed to read CA certificate file")
	errClientCertLoad              = errors.New("failed to load client certificate")
	errNotController               = errors.New("not a controller")
	errUnreachable                 = errors.New("unreachable")
)
