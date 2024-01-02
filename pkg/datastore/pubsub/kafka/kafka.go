/*
Package kafka provides methods to interact with Apache Kafka offering functionality for both producing and consuming
messages from kafka-topics.
*/
package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"

	"golang.org/x/net/context"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/datastore/pubsub/avro"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

const (
	errConsumeMsg       = errors.Error("error while consuming the message")
	SASLTypeSCRAMSHA512 = "SCRAM-SHA-512"
	PLAIN               = "PLAIN"
	errInvalidMechanism = errors.Error("Invalid SASL Mechanism")
)

// Kafka is a client for interacting with Apache Kafka.
type Kafka struct {
	config   *Config
	logger   log.Logger
	Producer sarama.SyncProducer
	Consumer *Consumer
}

// AvroWithKafkaConfig represents a configuration for using Avro with Kafka
type AvroWithKafkaConfig struct {
	KafkaConfig Config
	AvroConfig  avro.Config
}

const (
	// OffsetNewest stands for the log head offset, i.e. the offset that will be
	// assigned to the next message that will be produced to the partition.
	OffsetNewest int64 = sarama.OffsetNewest
	// OffsetOldest stands for the oldest offset available on the broker for a
	// partition. You can send this to a client's GetOffset method to get this
	// offset, or when calling ConsumePartition to start consuming from the
	// oldest offset that is still available on the broker.
	OffsetOldest int64 = sarama.OffsetOldest
)

// Consumer is a wrapper on sarama ConsumerGroup.
type Consumer struct {
	ConsumerGroup        sarama.ConsumerGroup
	ConsumerGroupHandler *ConsumerHandler
	// initSessionRebalance is used to maintain the state of the consumer
	// in a consumer group to check if the partition rebalance  has been
	// initiated on this consumer session. The Consume method must be called
	// only once in a go routine, as an infinite loop, which is ensured by
	// using this field.
	initSessionRebalance sync.Once
	errCh                <-chan error
}

// Config provide values for kafka producer and consumer
type Config struct {
	// Brokers comma separated kafka brokers
	Brokers string

	// SASL provide configs for authentication
	SASL *SASLConfig

	// MaxRetry number of times to retry sending a failing message
	MaxRetry int

	// RetryFrequency backoff time in milliseconds before retrying
	RetryFrequency int

	// Topics multiple topics to subscribe messages from
	// first topic will be used for publishing the message
	Topics []string

	// GroupID consumer group id
	GroupID string

	Config *sarama.Config

	// ConnRetryDuration for specifying connection retry duration
	ConnRetryDuration int

	// Offsets is slice of TopicPartition in which "Topic","Partition" and "Offset"
	// are the field needed to be set to start consuming from specific offset
	Offsets []pubsub.TopicPartition

	InitialOffsets int64

	// This config will allow application to disable kafka consumer auto commit
	DisableAutoCommit bool
}

// SASLConfig holds SASL authentication configurations for Kafka.
type SASLConfig struct {
	// User username to connect to protected kafka instance
	User string

	// Password password to connect to protected kafka instance
	Password string

	// Mechanism SASL mechanism used for authentication
	Mechanism string

	// SecurityProtocol SSL or PLAINTEXT
	SecurityProtocol string

	// SSLVerify set it to true if certificate verification is required
	SSLVerify bool

	// CertificateFile for TLS client authentication
	CertificateFile string

	// KeyFile for TLS client authentication
	KeyFile string

	// CACertificateFile is the certificate authority file for TLS client authentication
	CACertificateFile string
}

// NewKafkaFromEnv fetches the config from environment variables and tries to connect to Kafka
// Deprecated: Instead use pubsub.New
func NewKafkaFromEnv() (*Kafka, error) {
	hosts := os.Getenv("KAFKA_HOSTS") // CSV string
	topic := os.Getenv("KAFKA_TOPIC") // CSV string
	user := os.Getenv("KAFKA_SASL_USER")
	password := os.Getenv("KAFKA_SASL_PASS")
	mechanism := os.Getenv("KAFKA_SASL_MECHANISM")

	// converting the CSV string to slice of string
	topics := strings.Split(topic, ",")

	config := &Config{
		Brokers: hosts,
		SASL: &SASLConfig{
			User:      user,
			Password:  password,
			Mechanism: mechanism,
		},
		Topics: topics,
	}

	producer, err := NewKafkaProducer(config)
	if err != nil {
		return nil, err
	}

	consumer, err := NewKafkaConsumer(config)
	if err != nil {
		return nil, err
	}

	kafkaObj := &Kafka{
		config:   config,
		Producer: producer,
		Consumer: consumer,
	}

	err = kafkaObj.Ping()
	if err != nil {
		return nil, err
	}

	return kafkaObj, nil
}

// New establishes connection to Kafka using the config provided in KafkaConfig
func New(config *Config, logger log.Logger) (*Kafka, error) {
	pubsub.RegisterMetrics()

	if config.SASL.Mechanism != SASLTypeSCRAMSHA512 && config.SASL.Mechanism != PLAIN && config.SASL.User != "" {
		return nil, errInvalidMechanism
	}

	populateOffsetTopic(config)
	convertKafkaConfig(config)

	sarama.Logger = kafkaLogger{logger: logger}

	brokers := strings.Split(config.Brokers, ",")

	p, err := sarama.NewSyncProducer(brokers, config.Config)
	if err != nil {
		return &Kafka{config: config, logger: logger}, err
	}

	c, err := NewKafkaConsumer(config)
	if err != nil {
		return &Kafka{config: config, logger: logger}, err
	}

	kafkaObj := &Kafka{config: config, logger: logger, Producer: p, Consumer: c}

	if err := kafkaObj.Ping(); err != nil {
		return &Kafka{config: config, logger: logger}, err
	}

	return kafkaObj, nil
}

// populateOffsetTopic populate the Offsets Topic field with the topic provided in kafka config.
func populateOffsetTopic(c *Config) {
	var topic string
	if c.Topics != nil {
		topic = c.Topics[0]
	}

	for i, v := range c.Offsets {
		if v.Topic == "" {
			c.Offsets[i].Topic = topic
		}
	}
}

// NewKafkaProducer returns a kafka producer object created	using the configs provided. returns error if configs are invalid
func NewKafkaProducer(config *Config) (sarama.SyncProducer, error) {
	convertKafkaConfig(config)
	brokers := strings.Split(config.Brokers, ",")

	return sarama.NewSyncProducer(brokers, config.Config)
}

// NewKafkaConsumer returns a kafka consumer object created using the configs provided. returns error if configs are invalid
func NewKafkaConsumer(config *Config) (*Consumer, error) {
	convertKafkaConfig(config)
	brokers := strings.Split(config.Brokers, ",")

	cg, err := sarama.NewConsumerGroup(brokers, config.GroupID, config.Config)
	if err != nil {
		return nil, err
	}

	consumerHandler := ConsumerHandler{
		msg:     make(chan *sarama.ConsumerMessage),
		ready:   make(chan bool),
		offsets: config.Offsets,
	}

	errCh := make(<-chan error)

	return &Consumer{
		ConsumerGroup:        cg,
		ConsumerGroupHandler: &consumerHandler,
		errCh:                errCh,
	}, nil
}

func convertKafkaConfig(config *Config) {
	if config == nil {
		// default new kafka config to 3 retries(default for sarama kafka). We do it here,
		// because we also want to give the user the ability to set it
		// to 0 retries elsewhere.
		maxRetries := 3
		config = &Config{MaxRetry: maxRetries}
	}

	if config.Config == nil {
		config.Config = sarama.NewConfig()
	}

	config.Config.Version = sarama.MaxVersion

	setDefaultConfig(config.Config)

	processSASLConfigs(config.SASL, config.Config)

	if config.InitialOffsets != 0 {
		config.Config.Consumer.Offsets.Initial = config.InitialOffsets
	}

	if config.MaxRetry >= 0 {
		config.Config.Producer.Retry.Max = config.MaxRetry
	}

	if config.DisableAutoCommit {
		config.Config.Consumer.Offsets.AutoCommit.Enable = false
	}

	if config.RetryFrequency > 0 {
		config.Config.Producer.Retry.Backoff = time.Duration(config.RetryFrequency)
	}

	// for logging purpose only
	if config.Config.ClientID == "" {
		config.Config.ClientID = "gofr-kafka-log"
	}

	if config.GroupID == "" {
		config.GroupID = "gofr-consumerGroup"
	}

	if config.Config.Consumer.Group.Member.UserData == nil {
		config.Config.Consumer.Group.Member.UserData = []byte(config.GroupID)
	}
}

func setDefaultConfig(config *sarama.Config) {
	if config.Metadata.Retry.Backoff == 0 {
		config.Metadata.Retry.Backoff = 100000000
	}

	if config.Metadata.Retry.Max == 0 {
		config.Metadata.Retry.Max = 5
	}

	if config.Producer.Retry.Backoff == 0 {
		config.Producer.Retry.Backoff = 5000000
	}

	if config.Producer.Timeout == 0 {
		config.Producer.Timeout = 300000000
	}

	if config.Producer.RequiredAcks == 0 {
		config.Producer.RequiredAcks = 1
	}

	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	if config.Consumer.Retry.Backoff == 0 {
		config.Consumer.Retry.Backoff = 5000000
	}

	if config.Consumer.MaxWaitTime == 0 {
		config.Consumer.MaxWaitTime = 300000000
	}

	if config.Consumer.Group.Heartbeat.Interval == 0 {
		config.Consumer.Group.Heartbeat.Interval = 1000000
	}
}

func processSASLConfigs(s *SASLConfig, conf *sarama.Config) {
	if s.User != "" && s.Password != "" {
		conf.Net.SASL.Enable = true
		conf.Net.SASL.User = s.User
		conf.Net.SASL.Password = s.Password
		conf.Net.SASL.Handshake = true

		if s.SSLVerify {
			conf.Net.TLS.Enable = true
			tlsConf, err := createTLSConfiguration(s.CACertificateFile, s.CertificateFile, s.KeyFile)

			if err != nil {
				return
			}

			conf.Net.TLS.Config = tlsConf
		}

		switch s.Mechanism {
		case SASLTypeSCRAMSHA512:
			conf.Net.SASL.Mechanism = sarama.SASLMechanism(s.Mechanism)
			conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
			}
		case PLAIN:
			conf.Net.SASL.Mechanism = sarama.SASLMechanism(s.Mechanism)
		}
	}
}

// createTLSConfiguration creates TLS configuration on the basis of provided configs
func createTLSConfiguration(caCertPath, clientCertPath, clientKeyPath string) (*tls.Config, error) {
	rootCertPool := x509.NewCertPool()

	pem, err := os.ReadFile(caCertPath)
	if err != nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}, err
	}

	if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
		return &tls.Config{MinVersion: tls.VersionTLS12}, err
	}

	clientCert := make([]tls.Certificate, 0, 1)

	certs, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}, err
	}

	clientCert = append(clientCert, certs)

	return &tls.Config{
		RootCAs:      rootCertPool,
		Certificates: clientCert,
		//nolint:gosec // cannot keep InsecureSkipVerify as false as one can use self signed certificates
		InsecureSkipVerify: true, // needed for self signed certs
	}, nil
}

// PublishEvent publishes the event to kafka
func (k *Kafka) PublishEvent(key string, value interface{}, headers map[string]string) (err error) {
	return k.PublishEventWithOptions(key, value, headers, &pubsub.PublishOptions{
		Topic:     k.config.Topics[0],
		Timestamp: time.Now(),
	})
}

// PublishEventWithOptions publishes message to kafka. Ability to provide additional options described in PublishOptions struct
func (k *Kafka) PublishEventWithOptions(key string, value interface{}, headers map[string]string,
	options *pubsub.PublishOptions) (err error) {
	if options == nil {
		options = &pubsub.PublishOptions{}
	}

	if options.Topic == "" {
		options.Topic = k.config.Topics[0]
	}

	if options.Timestamp.IsZero() {
		options.Timestamp = time.Now()
	}

	pubsub.PublishTotalCount(options.Topic, k.config.GroupID)

	valBytes, ok := value.([]byte)
	if !ok {
		valBytes, err = json.Marshal(value)
		if err != nil {
			pubsub.PublishFailureCount(options.Topic, k.config.GroupID)

			return err
		}
	}

	kafkaHeaders := make([]sarama.RecordHeader, 0, len(headers))

	for key, value := range headers {
		kafkaHeaders = append(kafkaHeaders, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
	}

	message := &sarama.ProducerMessage{
		Topic:     options.Topic,
		Partition: int32(options.Partition),
		Value:     sarama.ByteEncoder(valBytes),
		Key:       sarama.StringEncoder(key),
		Timestamp: options.Timestamp,
		Headers:   kafkaHeaders,
	}

	_, _, err = k.Producer.SendMessage(message)
	if err != nil {
		pubsub.PublishFailureCount(message.Topic, k.config.GroupID)

		return err
	}

	pubsub.PublishSuccessCount(message.Topic, k.config.GroupID)

	return nil
}

// rebalanceSession this method is responsible for rebalancing partitions
// allocation whenever a new consumer is spawned, or an old one is closed.
// It runs as an infinite for loop inside a goroutine.
func (c *Consumer) rebalanceSession(ctx context.Context,
	config *Config) <-chan error {
	err := make(chan error)

	go func(consumerGroup sarama.ConsumerGroup, handler *ConsumerHandler) {
		for {
			if e := consumerGroup.Consume(ctx, config.Topics, handler); e != nil {
				err <- e
				return
			}

			if e := ctx.Err(); e != nil {
				err <- e
				return
			}

			handler.ready = make(chan bool)
		}
	}(c.ConsumerGroup, c.ConsumerGroupHandler)

	return err
}

func (k *Kafka) subscribeMessage() (*pubsub.Message, error) {
	k.Consumer.initSessionRebalance.Do(
		func() {
			k.Consumer.errCh = k.Consumer.rebalanceSession(context.TODO(), k.config)
		})

	select {
	case e := <-k.Consumer.errCh:
		return nil, e

	case <-k.Consumer.ConsumerGroupHandler.ready:
	}

	msg := <-k.Consumer.ConsumerGroupHandler.msg
	if msg == nil {
		return nil, errConsumeMsg
	}

	headers := make(map[string]string, len(msg.Headers))

	for _, v := range msg.Headers {
		if len(v.Key) != 0 && len(v.Value) != 0 {
			headers[string(v.Key)] = string(v.Value)
		}
	}

	// Mark the message as read for autocommit to work
	k.Consumer.ConsumerGroupHandler.mu.Lock()
	k.Consumer.ConsumerGroupHandler.consumerGroupSession.MarkMessage(msg, "")
	k.Consumer.ConsumerGroupHandler.mu.Unlock()

	return &pubsub.Message{
		Topic:     msg.Topic,
		Partition: int(msg.Partition),
		Offset:    msg.Offset,
		Key:       string(msg.Key),
		Value:     string(msg.Value),
		Headers:   headers,
	}, nil
}

// Subscribe method is responsible for consuming a single message from a
// sarama Consumer Group. When Subscribe is called first time we initiate
// consumer group session rebalance, which handles the partition assignment
// to multiple consumers in the group.
func (k *Kafka) Subscribe() (*pubsub.Message, error) {
	topics := strings.Join(k.config.Topics, ",")
	// for every subscribe
	pubsub.SubscribeReceiveCount(topics, k.config.GroupID)

	message, err := k.subscribeMessage()
	if err != nil {
		// for unsuccessful subscribe
		pubsub.SubscribeFailureCount(topics, k.config.GroupID)

		return nil, err
	}

	k.CommitOffset(pubsub.TopicPartition{
		Topic:     message.Topic,
		Partition: message.Partition,
		Offset:    message.Offset,
	})
	// for successful subscribe
	pubsub.SubscribeSuccessCount(message.Topic, k.config.GroupID)

	return message, nil
}

/*
SubscribeWithCommit calls the CommitFunc after subscribing message from kafka and based on the return values decides
whether to commit message and consume another message
*/
func (k *Kafka) SubscribeWithCommit(f pubsub.CommitFunc) (*pubsub.Message, error) {
	for {
		topics := strings.Join(k.config.Topics, ",")
		// for every subscribe
		pubsub.SubscribeReceiveCount(topics, k.config.GroupID)

		msg, err := k.subscribeMessage()
		if err != nil {
			// for unsuccessful subscribe
			pubsub.SubscribeFailureCount(topics, k.config.GroupID)

			return nil, err
		}

		// handle panic case if f is nil.
		// when avro is being used, we need to return the message to avro
		// avro will call f() after processing the message
		if f == nil {
			// for failed subscribe operation
			pubsub.SubscribeFailureCount(topics, k.config.GroupID)

			return msg, nil
		}

		isCommit, isContinue := f(msg)

		// for successful subscribe
		pubsub.SubscribeSuccessCount(topics, k.config.GroupID)

		if isCommit {
			k.CommitOffset(pubsub.TopicPartition{
				Topic:     msg.Topic,
				Partition: msg.Partition,
				Offset:    msg.Offset,
			})
		}

		if !isContinue {
			return msg, nil
		}
	}
}

// Bind parses the encoded data and stores the result in the value pointed to by target
func (k *Kafka) Bind(message []byte, target interface{}) error {
	return json.Unmarshal(message, target)
}

// Ping checks for the health of kafka, returns an error if it is down
func (k *Kafka) Ping() error {
	brokers := strings.Split(k.config.Brokers, ",")
	accessibleBrokers := make([]string, 0, len(brokers))

	for _, host := range brokers {
		// if we are able to dial to a connection then its possible to read and write into apache-kafka
		conn, err := net.Dial("tcp", host)
		if err != nil {
			break
		}

		err = conn.Close()
		if err != nil {
			return err
		}

		accessibleBrokers = append(accessibleBrokers, host)
	}

	if len(accessibleBrokers) == 0 {
		return brokersErr{}
	}

	return nil
}

// Pause suspends fetching from all partitions.
func (k *Kafka) Pause() error {
	k.Consumer.ConsumerGroup.PauseAll()
	return nil
}

// Resume resumes all partitions which have been paused with Pause()
func (k *Kafka) Resume() error {
	k.Consumer.ConsumerGroup.ResumeAll()
	return nil
}

type brokersErr struct{}

// Error returns error on connection failure
func (b brokersErr) Error() string {
	return "invalid brokers connection failed"
}

// ConsumerHandler represents a Sarama consumer group message handler.
// It is responsible for handling the messages in a specific topic/partition.
// The handler implements methods called during the lifecycle of a consumer
// group session.
type ConsumerHandler struct {
	msg                  chan *sarama.ConsumerMessage
	ready                chan bool
	consumerGroupSession sarama.ConsumerGroupSession
	offsets              []pubsub.TopicPartition
	initialiseOffset     sync.Once
	mu                   sync.Mutex
}

// setOffset set the starting offset of all the partition in the topic.
func setOffset(s sarama.ConsumerGroupSession, consumer *ConsumerHandler) {
	for _, v := range consumer.offsets {
		p := int32(v.Partition)
		// To start subscribing uncommitted message from specific offset
		s.MarkOffset(v.Topic, p, v.Offset, "")
		// To start subscribing committed message from specific offset
		s.ResetOffset(v.Topic, p, v.Offset, "")
	}
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *ConsumerHandler) Setup(s sarama.ConsumerGroupSession) error {
	// Set the initial custom offset value for subscribe
	if consumer.offsets != nil {
		consumer.initialiseOffset.Do(
			func() {
				setOffset(s, consumer)
			})
	}
	// Mark the consumer as ready
	close(consumer.ready)

	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *ConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *ConsumerHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim) error {
	// Following consumes messages from a particular topic/partition claims.
	// Once a message is consumed we mark the message as consumed.
	// The handler must exit as soon as a rebalance occurs. Listen to the
	// session Context and return from handler as soon as it is ended.
	for {
		select {
		case message := <-claim.Messages():
			consumer.mu.Lock()
			consumer.consumerGroupSession = session
			consumer.mu.Unlock()
			consumer.msg <- message

		case <-session.Context().Done():
			return nil
		}
	}
}

// kafkaLogger is used only for debugging purposes
// step by step logs are provided by sarama kafka for connection,
// publishing and subscribing messages
type kafkaLogger struct {
	logger log.Logger
}

// Print is used to display the text on console.
func (kl kafkaLogger) Print(v ...interface{}) {
	kl.logger.Debug(v...)
}

// Printf is used to display formatted output.
func (kl kafkaLogger) Printf(format string, v ...interface{}) {
	kl.logger.Debugf(format, v...)
}

// Println is used to display the text in new line.
func (kl kafkaLogger) Println(v ...interface{}) {
	kl.logger.Debug(v...)
}

// CommitOffset marks a particular offset on a specific partition as Read.
// The commits are performed asynchronously at intervals specified in Sarama
// Consumer.Offsets.AutoCommit
func (k *Kafka) CommitOffset(offsets pubsub.TopicPartition) {
	k.Consumer.ConsumerGroupHandler.mu.Lock()
	k.Consumer.ConsumerGroupHandler.consumerGroupSession.MarkOffset(
		offsets.Topic, int32(offsets.Partition), offsets.Offset+1, "")

	if k.config.DisableAutoCommit {
		k.Consumer.ConsumerGroupHandler.consumerGroupSession.Commit()
	}

	k.Consumer.ConsumerGroupHandler.mu.Unlock()
}

// HealthCheck checks if consumer and producer are initialized and the connection is stable
func (k *Kafka) HealthCheck() types.Health {
	if k == nil {
		return types.Health{
			Name:   datastore.Kafka,
			Status: pkg.StatusDown,
		}
	}

	database := strings.Join(k.config.Topics, ",")

	resp := types.Health{
		Name:     datastore.Kafka,
		Status:   pkg.StatusDown,
		Host:     k.config.Brokers,
		Database: database,
	}

	if k.Consumer == nil {
		k.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: datastore.Kafka, Reason: "Consumer is not initialized"})
		return resp
	}

	if k.Producer == nil {
		k.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: datastore.Kafka, Reason: "Producer is not initialized"})
		return resp
	}

	err := k.Ping()
	if err != nil {
		k.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: datastore.Kafka, Err: err})
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

// IsSet checks whether kafka is initialized or not
func (k *Kafka) IsSet() bool {
	if k == nil {
		return false
	}

	if k.Consumer == nil || k.Producer == nil {
		return false
	}

	return true
}

// NewKafkaWithAvro initialize Kafka with Avro when EventHubConfig and AvroConfig are right
func NewKafkaWithAvro(config *AvroWithKafkaConfig, logger log.Logger) (pubsub.PublisherSubscriber, error) {
	kafka, err := New(&config.KafkaConfig, logger)
	if err != nil {
		logger.Errorf("Kafka cannot be initialized, err: %v", err)
		return nil, err
	}

	p, err := avro.NewWithConfig(&config.AvroConfig, kafka)
	if err != nil {
		logger.Errorf("Avro cannot be initialized, err: %v", err)
		return nil, err
	}

	return p, nil
}
