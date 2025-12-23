package redis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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

	// PubSub constants.
	defaultRetryTimeout  = 10 * time.Second
	unsubscribeOpTimeout = 2 * time.Second
	goroutineWaitTimeout = 5 * time.Second
)

var (
	// PubSub errors.
	errClientNotConnected       = errors.New("redis client not connected")
	errEmptyTopicName           = errors.New("topic name cannot be empty")
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
	PubSubBufferSize    int           // Message buffer size for channels (default: 100)
	PubSubQueryTimeout  time.Duration // Default query timeout (default: 5s)
	PubSubQueryLimit    int           // Default query message limit (default: 10)
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
}

// PubSub handles Redis PubSub operations.
type PubSub struct {
	// Reference to Redis client connection
	client *redis.Client

	// Configuration, logger, and metrics
	config  *Config
	logger  datasource.Logger
	metrics Metrics

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

		go retryConnect(rc, logger)
	}

	r := &Redis{
		Client:  rc,
		config:  redisConfig,
		logger:  logger,
		metrics: metrics,
	}

	return r
}

// retryConnect handles the retry mechanism for connecting to Redis.
func retryConnect(client *redis.Client, logger datasource.Logger) {
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
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}

	return nil
}

// HealthCheck returns the health status of the Redis connection.
func (r *Redis) HealthCheck() datasource.Health {
	res := datasource.Health{
		Status: datasource.StatusDown,
		Details: map[string]any{
			"backend": "REDIS",
		},
	}

	if r.Client == nil {
		res.Details["error"] = "client not initialized"
		return res
	}

	addr := fmt.Sprintf("%s:%d", r.config.HostName, r.config.Port)
	res.Details["addr"] = addr
	res.Details["db"] = r.config.DB

	ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
	defer cancel()

	if err := r.Client.Ping(ctx).Err(); err != nil {
		r.logger.Errorf("Redis health check failed: %v", err)
		res.Details["error"] = err.Error()

		return res
	}

	res.Status = datasource.StatusUp

	return res
}

// NewPubSub creates a new PubSub client that implements pubsub.Client interface.
// This allows Redis PubSub to be initialized directly without type assertion,
// aligning with the pattern used by Kafka, MQTT, and Google PubSub implementations.
func NewPubSub(conf config.Config, logger datasource.Logger, metrics Metrics) pubsub.Client {
	redisConfig := getRedisConfig(conf, logger)

	// Always parse PubSub config for NewPubSub since we're creating a PubSub client
	// This ensures defaults are set even if PUBSUB_BACKEND wasn't explicitly set
	if !strings.EqualFold(conf.Get("PUBSUB_BACKEND"), "REDIS") {
		// If PUBSUB_BACKEND wasn't set, parse config anyway for NewPubSub
		parsePubSubConfig(conf, redisConfig)
	}

	// if Hostname is not provided, we won't try to connect to Redis
	if redisConfig.HostName == "" {
		return nil
	}

	// Allow PubSub to use a different Redis logical DB than the primary Redis datasource.
	// This prevents keyspace collisions (e.g., HASH vs STREAM on `gofr_migrations`) when Redis is used for both
	// migrations and PubSub (streams mode).
	//
	// If not set or invalid, we default to database 15 (highest default Redis database)
	// to avoid collisions with the main Redis database (typically 0).
	setPubSubDB(conf, redisConfig)

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

		go retryConnect(rc, logger)
	}

	ps := newPubSub(rc, redisConfig, logger, metrics)

	return ps
}

// setPubSubDB sets the PubSub database number from config or defaults to 15.
func setPubSubDB(conf config.Config, redisConfig *Config) {
	dbStr := conf.Get("REDIS_PUBSUB_DB")
	if dbStr == "" {
		redisConfig.DB = defaultPubSubDB
		if redisConfig.Options != nil {
			redisConfig.Options.DB = defaultPubSubDB
		}

		return
	}

	db, err := strconv.Atoi(dbStr)
	if err != nil || db < 0 {
		// Invalid value, use default
		redisConfig.DB = defaultPubSubDB
		if redisConfig.Options != nil {
			redisConfig.Options.DB = defaultPubSubDB
		}

		return
	}

	// Valid value, use it
	redisConfig.DB = db
	if redisConfig.Options != nil {
		redisConfig.Options.DB = db
	}
}

// newPubSub creates a new PubSub instance.
func newPubSub(client *redis.Client, redisCfg *Config, logger datasource.Logger, metrics Metrics) *PubSub {
	ps := &PubSub{
		client:          client,
		config:          redisCfg,
		logger:          logger,
		metrics:         metrics,
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

// TODO - if we make Redis an interface and expose from container we can avoid c.Redis(c, command) using methods on c and still pass c.
// type Redis interface {
//	Get(string) (string, error)
// }
