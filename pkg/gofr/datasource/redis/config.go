package redis

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	defaultRedisPort        = 6379
	defaultPubSubDB         = 15 // Highest default Redis database (0-15)
	modePubSub              = "pubsub"
	modeStreams             = "streams"
	defaultPubSubBufferSize = 100
	defaultPubSubQueryLimit = 10
)

const (
	defaultPubSubQueryTimeout = 5 * time.Second
)

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

// parsePubSubConfig parses PubSub configuration from environment variables.
func parsePubSubConfig(c config.Config, redisConfig *Config) {
	parsePubSubMode(c, redisConfig)
	parsePubSubCommonConfig(c, redisConfig)
}

// parsePubSubMode parses the PubSub mode configuration.
func parsePubSubMode(c config.Config, redisConfig *Config) {
	mode := strings.ToLower(c.Get("REDIS_PUBSUB_MODE"))
	if mode == "" {
		mode = modeStreams
	}

	if mode != modeStreams && mode != modePubSub {
		mode = modeStreams
	}

	redisConfig.PubSubMode = mode

	// Parse Streams config if mode is streams
	if mode == modeStreams {
		configStreams(c, redisConfig)
	}
}

// parsePubSubCommonConfig parses common PubSub configuration (buffer size, query timeout, query limit).
func parsePubSubCommonConfig(c config.Config, redisConfig *Config) {
	redisConfig.PubSubBufferSize = parseBufferSize(c)
	redisConfig.PubSubQueryTimeout = parseQueryTimeout(c)
	redisConfig.PubSubQueryLimit = parseQueryLimit(c)
}

// parseBufferSize parses the PubSub buffer size configuration.
func parseBufferSize(c config.Config) int {
	bufferSizeStr := c.Get("REDIS_PUBSUB_BUFFER_SIZE")
	if bufferSizeStr == "" {
		return defaultPubSubBufferSize
	}

	bufferSize, err := strconv.Atoi(bufferSizeStr)
	if err != nil || bufferSize <= 0 {
		return defaultPubSubBufferSize
	}

	return bufferSize
}

// parseQueryTimeout parses the PubSub query timeout configuration.
func parseQueryTimeout(c config.Config) time.Duration {
	timeoutStr := c.Get("REDIS_PUBSUB_QUERY_TIMEOUT")
	if timeoutStr == "" {
		return defaultPubSubQueryTimeout
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return defaultPubSubQueryTimeout
	}

	return timeout
}

// parseQueryLimit parses the PubSub query limit configuration.
func parseQueryLimit(c config.Config) int {
	limitStr := c.Get("REDIS_PUBSUB_QUERY_LIMIT")
	if limitStr == "" {
		return defaultPubSubQueryLimit
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultPubSubQueryLimit
	}

	return limit
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
