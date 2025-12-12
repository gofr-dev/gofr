package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	otel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	otelglobal "go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

const (
	redisPingTimeout = 5 * time.Second
	defaultRedisPort = 6379

	// PubSub constants.
	defaultRetryTimeout  = 10 * time.Second
	messageBufferSize    = 100
	unsubscribeOpTimeout = 2 * time.Second
	goroutineWaitTimeout = 5 * time.Second
)

var (
	// redisLogFilterOnce ensures we only set up the logger once.
	redisLogFilterOnce sync.Once //nolint:gochecknoglobals // This is a package-level singleton for logger setup
)

var (
	// PubSub errors.
	errClientNotConnected       = errors.New("redis client not connected")
	errEmptyTopicName           = errors.New("topic name cannot be empty")
	errPublisherNotConfigured   = errors.New("redis publisher not configured")
	errPubSubConnectionFailed   = errors.New("failed to create PubSub connection for query")
	errPubSubChannelFailed      = errors.New("failed to get channel from PubSub for query")
	errConsumerGroupNotProvided = errors.New("consumer group must be provided for streams mode")
)

type Config struct {
	HostName string
	Username string
	Password string
	Port     int
	DB       int
	Options  *redis.Options
	TLS      *tls.Config

	// PubSub configuration
	PubSubMode          string // "pubsub" or "streams"
	PubSubStreamsConfig *StreamsConfig
}

// StreamsConfig holds configuration for Redis Streams.
type StreamsConfig struct {
	// ConsumerGroup is the name of the consumer group (required for Streams)
	ConsumerGroup string

	// ConsumerName is the name of the consumer (optional, auto-generated if empty)
	ConsumerName string

	// MaxLen is the maximum length of the stream (optional)
	// If > 0, the stream will be trimmed to this length on publish
	MaxLen int64

	// Block is the blocking duration for XREADGROUP (optional)
	// If > 0, calls will block for this duration waiting for new messages
	Block time.Duration
}

type Redis struct {
	*redis.Client
	logger  datasource.Logger
	config  *Config
	metrics Metrics

	// PubSub for Redis PubSub operations (separate struct, not embedded to avoid method conflicts)
	PubSub *PubSub
}

// PubSub handles Redis PubSub operations, reusing the parent Redis connection.
type PubSub struct {
	// Reference to parent Redis client connection (reused, not duplicated)
	client *redis.Client

	// Parent Redis for accessing config, logger, metrics
	// parent.logger: Logger instance from the parent Redis client for logging operations
	// parent.metrics: Metrics instance from the parent Redis client for recording metrics
	// parent.config: Configuration from the parent Redis client (includes PubSubMode, StreamsConfig, etc.)
	parent *Redis

	// Tracer for OpenTelemetry distributed tracing
	tracer oteltrace.Tracer

	// Subscription management
	receiveChan     map[string]chan *pubsub.Message
	subStarted      map[string]struct{}
	subCancel       map[string]context.CancelFunc
	subPubSub       map[string]*redis.PubSub // Track active PubSub connections for unsubscribe
	subWg           map[string]*sync.WaitGroup
	chanClosed      map[string]bool
	streamConsumers map[string]*streamConsumer
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

// streamConsumer represents a consumer in a Redis Stream consumer group.
type streamConsumer struct {
	stream   string
	group    string
	consumer string
	cancel   context.CancelFunc
}

// NewClient returns a [Redis] client if connection is successful based on [Config].
// Supports both plain and TLS connections. TLS is configured via REDIS_TLS_ENABLED and related environment variables.
// In case of error, it returns an error as second parameter.
func NewClient(c config.Config, logger datasource.Logger, metrics Metrics) *Redis {
	redisConfig := getRedisConfig(c, logger)

	// if Hostname is not provided, we won't try to connect to Redis
	if redisConfig.HostName == "" {
		return nil
	}

	logger.Debugf("connecting to redis at '%s:%d' on database %d", redisConfig.HostName, redisConfig.Port, redisConfig.DB)

	// Redirect go-redis internal logs to Gofr logger for consistent formatting
	// go-redis v9 supports SetLogger to customize logging
	redisLogFilterOnce.Do(func() {
		redis.SetLogger(&gofrRedisLogger{logger: logger})
	})

	rc := redis.NewClient(redisConfig.Options)
	rc.AddHook(&redisHook{config: redisConfig, logger: logger, metrics: metrics})

	ctx, cancel := context.WithTimeout(context.TODO(), redisPingTimeout)
	defer cancel()

	if err := rc.Ping(ctx).Err(); err == nil {
		if err = otel.InstrumentTracing(rc); err != nil {
			logger.Errorf("could not add tracing instrumentation, error: %s", err)
		}

		logger.Infof("connected to redis at %s:%d on database %d", redisConfig.HostName, redisConfig.Port, redisConfig.DB)
	} else {
		logger.Errorf("could not connect to redis at '%s:%d' , error: %s", redisConfig.HostName, redisConfig.Port, err)

		go retryConnect(rc, redisConfig, logger)
	}

	r := &Redis{
		Client:  rc,
		config:  redisConfig,
		logger:  logger,
		metrics: metrics,
	}

	// Initialize PubSub if PUBSUB_BACKEND=REDIS
	pubsubBackend := c.Get("PUBSUB_BACKEND")

	if strings.EqualFold(pubsubBackend, "REDIS") {
		logger.Debug("PUBSUB_BACKEND is set to REDIS, initializing PubSub")

		r.PubSub = newPubSub(r, rc)
	} else {
		logger.Debug("PubSub not initialized because PUBSUB_BACKEND is not REDIS")
	}

	return r
}

// retryConnect handles the retry mechanism for connecting to Redis.
func retryConnect(client *redis.Client, _ *Config, logger datasource.Logger) {
	for {
		time.Sleep(defaultRetryTimeout)

		ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
		err := client.Ping(ctx).Err()

		cancel()

		if err == nil {
			if err = otel.InstrumentTracing(client); err != nil {
				logger.Errorf("could not add tracing instrumentation, error: %s", err)
			}

			logger.Info("connected to redis successfully")

			return
		}

		logger.Errorf("could not connect to redis, error: %s", err)
	}
}

// Close shuts down the Redis client, ensuring the current dataset is saved before exiting.
// Also closes PubSub if it was initialized.
func (r *Redis) Close() error {
	var err error

	if r.PubSub != nil {
		err = r.PubSub.Close()
	}

	if r.Client != nil {
		err = errors.Join(err, r.Client.Close())
	}

	return err
}

// getRedisConfig builds the Redis Config struct from the provided [Config].
// It supports TLS configuration using the following environment variables:
//
//	REDIS_TLS_ENABLED: set to "true" to enable TLS
//	REDIS_TLS_CA_CERT: PEM-encoded CA certificate (string)
//	REDIS_TLS_CERT:    PEM-encoded client certificate (string or file path)
//	REDIS_TLS_KEY:     PEM-encoded client private key (string or file path)
//
// If TLS is enabled, the function sets up the [tls.Config] for the [Redis] client.
func getRedisConfig(c config.Config, logger datasource.Logger) *Config {
	var redisConfig = &Config{}

	redisConfig.HostName = c.Get("REDIS_HOST")
	redisConfig.Username = c.Get("REDIS_USER")
	redisConfig.Password = c.Get("REDIS_PASSWORD")

	port, err := strconv.Atoi(c.Get("REDIS_PORT"))
	if err != nil {
		port = defaultRedisPort
	}

	redisConfig.Port = port

	db, err := strconv.Atoi(c.Get("REDIS_DB"))
	if err != nil {
		db = 0 // default to DB 0 if not specified
	}

	redisConfig.DB = db

	options := new(redis.Options)
	options.Addr = fmt.Sprintf("%s:%d", redisConfig.HostName, redisConfig.Port)
	options.Username = redisConfig.Username
	options.Password = redisConfig.Password
	options.DB = redisConfig.DB

	// Parse PubSub config if PUBSUB_BACKEND=REDIS
	if strings.EqualFold(c.Get("PUBSUB_BACKEND"), "REDIS") {
		parsePubSubConfig(c, redisConfig)
	}

	if c.Get("REDIS_TLS_ENABLED") != "true" {
		redisConfig.Options = options
		return redisConfig
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if caCertPath := c.Get("REDIS_TLS_CA_CERT"); caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			logger.Errorf("failed to read CA cert file: %v", err)
		} else {
			initializeCerts(logger, caCert, tlsConfig)
		}
	}

	// Load client cert and key from file paths
	certPath := c.Get("REDIS_TLS_CERT")
	keyPath := c.Get("REDIS_TLS_KEY")

	if certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			logger.Errorf("failed to load client cert/key pair: %v", err)
		} else {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	options.TLSConfig = tlsConfig
	redisConfig.TLS = tlsConfig
	redisConfig.Options = options

	return redisConfig
}

// newPubSub creates a new PubSub instance that reuses the parent Redis connection.
func newPubSub(parent *Redis, client *redis.Client) *PubSub {
	ps := &PubSub{
		client:          client,
		parent:          parent,
		tracer:          otelglobal.GetTracerProvider().Tracer("gofr"),
		receiveChan:     make(map[string]chan *pubsub.Message),
		subStarted:      make(map[string]struct{}),
		subCancel:       make(map[string]context.CancelFunc),
		subPubSub:       make(map[string]*redis.PubSub),
		subWg:           make(map[string]*sync.WaitGroup),
		chanClosed:      make(map[string]bool),
		streamConsumers: make(map[string]*streamConsumer),
	}

	ps.ctx, ps.cancel = context.WithCancel(context.Background())
	go ps.monitorConnection(ps.ctx)

	return ps
}

// parsePubSubConfig parses PubSub configuration from environment variables.
func parsePubSubConfig(c config.Config, redisConfig *Config) {
	// Parse mode (default: streams)
	mode := c.Get("REDIS_PUBSUB_MODE")
	if mode == "" {
		mode = modeStreams
	}

	redisConfig.PubSubMode = mode

	// Parse Streams config if mode is streams
	if mode == modeStreams {
		configStreams(c, redisConfig)
	}
}

func configStreams(c config.Config, redisConfig *Config) {
	streamsConfig := &StreamsConfig{
		ConsumerGroup: c.Get("REDIS_STREAMS_CONSUMER_GROUP"),
		ConsumerName:  c.Get("REDIS_STREAMS_CONSUMER_NAME"),
	}

	streamsConfig.Block = 5 * time.Second // default
	if blockStr := c.Get("REDIS_STREAMS_BLOCK_TIMEOUT"); blockStr != "" {
		if block, err := time.ParseDuration(blockStr); err == nil {
			streamsConfig.Block = block
		}
	}

	if maxLenStr := c.Get("REDIS_STREAMS_MAXLEN"); maxLenStr != "" {
		if maxLen, err := strconv.ParseInt(maxLenStr, 10, 64); err == nil {
			streamsConfig.MaxLen = maxLen
		}
	}

	redisConfig.PubSubStreamsConfig = streamsConfig
}

func initializeCerts(logger datasource.Logger, caCert []byte, tlsConfig *tls.Config) {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		logger.Errorf("failed to append CA cert to pool")
	} else {
		tlsConfig.RootCAs = caCertPool
	}
}

// gofrRedisLogger implements redis.Logger interface to redirect go-redis logs to Gofr logger.
type gofrRedisLogger struct {
	logger datasource.Logger
}

// Printf implements redis.Logger interface.
func (l *gofrRedisLogger) Printf(_ context.Context, format string, v ...any) {
	if l.logger != nil {
		// Format the message
		msg := fmt.Sprintf(format, v...)
		// Log through Gofr logger as DEBUG level
		// Connection pool retry attempts are logged here, while actual connection failures
		// are already logged by Gofr at ERROR level in NewClient/retryConnect
		l.logger.Debugf("%s", msg)
	}
}

// TODO - if we make Redis an interface and expose from container we can avoid c.Redis(c, command) using methods on c and still pass c.
// type Redis interface {
//	Get(string) (string, error)
// }
