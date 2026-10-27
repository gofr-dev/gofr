package redis

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	defaultRedisPort          = 6379
	defaultPubSubDB           = 15 // Highest default Redis database (0-15)
	modePubSub                = "pubsub"
	modeStreams               = "streams"
	defaultPubSubBufferSize   = 100
	defaultPubSubQueryLimit   = 10
	defaultPubSubQueryTimeout = 5 * time.Second
	redisBackend              = "REDIS"
	trueValue                 = "true"
	payloadKey                = "payload"
	queryPipeline             = "pipeline"
	redisURLPlaceholder       = "REDIS_URL"
	redisDBKey                = "REDIS_DB"
	redisHostKey              = "REDIS_HOST"
	redisPortKey              = "REDIS_PORT"
	redisUserKey              = "REDIS_USER"
	redisPasswordKey          = "REDIS_PASSWORD"
	redisTLSEnabledKey        = "REDIS_TLS_ENABLED"
)

var (
	errInvalidRedisURL       = errors.New("invalid REDIS_URL")
	errInvalidRedisURLScheme = errors.New("scheme must be redis or rediss")
	errInvalidRedisURLEscape = errors.New("invalid URL escape")
	errInvalidRedisURLHost   = errors.New("missing host")
)

// getRedisConfig builds the Redis Config struct from the provided [Config].
// When REDIS_URL is set it alone decides host, port, user, password, DB and TLS (see [configFromURL]);
// otherwise the discrete REDIS_* variables are used. It supports TLS configuration using
// the following environment variables:
//
//	REDIS_TLS_ENABLED: set to "true" to enable TLS (ignored when REDIS_URL is set; use rediss:// instead)
//	REDIS_TLS_CA_CERT: PEM-encoded CA certificate (string)
//	REDIS_TLS_CERT:    PEM-encoded client certificate (string or file path)
//	REDIS_TLS_KEY:     PEM-encoded client private key (string or file path)
//
// If TLS is enabled, the function sets up the [tls.Config] for the [Redis] client.
// Pool size and timeouts are then applied from REDIS_POOL_SIZE, REDIS_MIN_IDLE_CONNS and
// REDIS_DIAL_TIMEOUT / REDIS_READ_TIMEOUT / REDIS_WRITE_TIMEOUT.
func getRedisConfig(c config.Config, logger datasource.Logger) *Config {
	var redisConfig *Config

	if rawURL := c.Get("REDIS_URL"); rawURL != "" {
		redisConfig = configFromURL(c, rawURL, logger)
		if redisConfig.HostName == "" {
			return redisConfig
		}
	} else {
		redisConfig = configFromVars(c, logger)
	}

	applyPoolOptions(c, redisConfig.Options, logger)

	// Parse PubSub config if PUBSUB_BACKEND=REDIS
	if strings.EqualFold(c.Get("PUBSUB_BACKEND"), redisBackend) {
		parsePubSubConfig(c, redisConfig)
	}

	return redisConfig
}

// configFromURL builds the connection settings from REDIS_URL. On an invalid URL it logs an
// error and returns a Config with an empty HostName, so no client is created.
func configFromURL(c config.Config, rawURL string, logger datasource.Logger) *Config {
	warnURLReplacedVars(c, logger)

	redisConfig, err := parseRedisURL(rawURL)
	if err != nil {
		logger.Errorf("%v", err)

		return &Config{}
	}

	if redisConfig.TLS != nil {
		applyTLSFiles(c, redisConfig.TLS, logger)
	}

	return redisConfig
}

// parseRedisURL parses a redis:// or rediss:// URL. The returned error never contains the raw URL.
func parseRedisURL(rawURL string) (*Config, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, redactURLError(err, rawURL)
	}

	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return nil, fmt.Errorf("%w: %w, got %q", errInvalidRedisURL, errInvalidRedisURLScheme, u.Scheme)
	}

	// go-redis turns an empty host into localhost, so redis://:6379 would connect to the wrong server.
	if u.Hostname() == "" {
		return nil, fmt.Errorf("%w: %w", errInvalidRedisURL, errInvalidRedisURLHost)
	}

	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, redactURLError(err, rawURL)
	}

	host, portStr, err := net.SplitHostPort(options.Addr)
	if err != nil {
		return nil, redactURLError(err, rawURL)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, redactURLError(err, rawURL)
	}

	return &Config{
		HostName: host,
		Port:     port,
		Username: options.Username,
		Password: options.Password,
		DB:       options.DB,
		Options:  options,
		TLS:      options.TLSConfig,
	}, nil
}

// redactURLError strips the raw URL, which may hold a password, from a parse error.
func redactURLError(err error, rawURL string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}

	// An escape error quotes the bad sequence, which can be part of the password.
	var escapeErr url.EscapeError
	if errors.As(err, &escapeErr) {
		return fmt.Errorf("%w: %w", errInvalidRedisURL, errInvalidRedisURLEscape)
	}

	return fmt.Errorf("%w: %s", errInvalidRedisURL, strings.ReplaceAll(err.Error(), rawURL, redisURLPlaceholder))
}

// warnURLReplacedVars logs the names, never the values, of set variables that REDIS_URL replaces.
func warnURLReplacedVars(c config.Config, logger datasource.Logger) {
	// The variables that REDIS_URL replaces when it is set.
	urlReplacedVars := []string{redisHostKey, redisPortKey, redisUserKey, redisPasswordKey, redisDBKey, redisTLSEnabledKey}

	var ignored []string

	for _, name := range urlReplacedVars {
		if c.Get(name) != "" {
			ignored = append(ignored, name)
		}
	}

	if len(ignored) > 0 {
		logger.Warnf("REDIS_URL is set, ignoring %s", strings.Join(ignored, ", "))
	}
}

// applyPoolOptions sets pool size and timeouts from their variables, overriding any REDIS_URL query parameter.
// An invalid value logs a warning and leaves the option as it was. Pool size and timeouts must be positive:
// go-redis turns 0 into its own default, which would silently discard a value set in REDIS_URL.
func applyPoolOptions(c config.Config, options *redis.Options, logger datasource.Logger) {
	if v, ok := intAtLeast(c, "REDIS_POOL_SIZE", 1, logger); ok {
		options.PoolSize = v
	}

	if v, ok := intAtLeast(c, "REDIS_MIN_IDLE_CONNS", 0, logger); ok {
		options.MinIdleConns = v
	}

	if d, ok := positiveDuration(c, "REDIS_DIAL_TIMEOUT", logger); ok {
		options.DialTimeout = d
	}

	if d, ok := positiveDuration(c, "REDIS_READ_TIMEOUT", logger); ok {
		options.ReadTimeout = d
	}

	if d, ok := positiveDuration(c, "REDIS_WRITE_TIMEOUT", logger); ok {
		options.WriteTimeout = d
	}
}

func intAtLeast(c config.Config, name string, minValue int, logger datasource.Logger) (int, bool) {
	raw := c.Get(name)
	if raw == "" {
		return 0, false
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v < minValue {
		logger.Warnf("invalid %s %q, want an integer of at least %d; ignoring it", name, raw, minValue)

		return 0, false
	}

	return v, true
}

func positiveDuration(c config.Config, name string, logger datasource.Logger) (time.Duration, bool) {
	raw := c.Get(name)
	if raw == "" {
		return 0, false
	}

	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		logger.Warnf("invalid %s %q, want a positive duration such as 500ms; ignoring it", name, raw)

		return 0, false
	}

	return d, true
}

// addr returns the host and port as a dialable address.
func (c *Config) addr() string {
	return net.JoinHostPort(c.HostName, strconv.Itoa(c.Port))
}

// configFromVars builds the connection settings from the discrete REDIS_* variables.
func configFromVars(c config.Config, logger datasource.Logger) *Config {
	var redisConfig = &Config{}

	redisConfig.HostName = c.Get(redisHostKey)
	redisConfig.Username = c.Get(redisUserKey)
	redisConfig.Password = c.Get(redisPasswordKey)

	port, err := strconv.Atoi(c.Get(redisPortKey))
	if err != nil {
		port = defaultRedisPort
	}

	redisConfig.Port = port

	db, err := strconv.Atoi(c.Get(redisDBKey))
	if err != nil {
		db = 0 // default to DB 0 if not specified
	}

	redisConfig.DB = db

	options := new(redis.Options)
	options.Addr = redisConfig.addr()
	options.Username = redisConfig.Username
	options.Password = redisConfig.Password
	options.DB = redisConfig.DB
	redisConfig.Options = options

	if c.Get(redisTLSEnabledKey) != trueValue {
		return redisConfig
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	applyTLSFiles(c, tlsConfig, logger)

	options.TLSConfig = tlsConfig
	redisConfig.TLS = tlsConfig

	return redisConfig
}

// applyTLSFiles loads REDIS_TLS_CA_CERT and the REDIS_TLS_CERT / REDIS_TLS_KEY pair into tlsConfig.
func applyTLSFiles(c config.Config, tlsConfig *tls.Config, logger datasource.Logger) {
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
}

// parsePubSubConfig parses PubSub configuration from environment variables.
func parsePubSubConfig(c config.Config, redisConfig *Config) {
	parsePubSubMode(c, redisConfig)
	parsePubSubCommonConfig(c, redisConfig)
}

// parsePubSubMode parses the PubSub mode configuration.
func parsePubSubMode(c config.Config, redisConfig *Config) {
	mode := strings.ToLower(c.GetOrDefault("REDIS_PUBSUB_MODE", modeStreams))

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
	// Buffer Size
	bufferSizeStr := c.GetOrDefault("REDIS_PUBSUB_BUFFER_SIZE", strconv.Itoa(defaultPubSubBufferSize))

	bufferSize, err := strconv.Atoi(bufferSizeStr)
	if err != nil || bufferSize <= 0 {
		redisConfig.PubSubBufferSize = defaultPubSubBufferSize
	} else {
		redisConfig.PubSubBufferSize = bufferSize
	}

	// Query Timeout
	timeoutStr := c.GetOrDefault("REDIS_PUBSUB_QUERY_TIMEOUT", defaultPubSubQueryTimeout.String())

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		redisConfig.PubSubQueryTimeout = defaultPubSubQueryTimeout
	} else {
		redisConfig.PubSubQueryTimeout = timeout
	}

	// Query Limit
	limitStr := c.GetOrDefault("REDIS_PUBSUB_QUERY_LIMIT", strconv.Itoa(defaultPubSubQueryLimit))

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		redisConfig.PubSubQueryLimit = defaultPubSubQueryLimit
	} else {
		redisConfig.PubSubQueryLimit = limit
	}
}

func configStreams(c config.Config, redisConfig *Config) {
	streamsConfig := &StreamsConfig{
		ConsumerGroup: c.Get("REDIS_STREAMS_CONSUMER_GROUP"),
		ConsumerName:  c.Get("REDIS_STREAMS_CONSUMER_NAME"),
		PELRatio:      0.7, // Default: 70% PEL, 30% new messages
	}

	streamsConfig.Block = 1 * time.Second // default - reduced from 5s for better responsiveness

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

	// Parse PEL ratio (0.0-1.0)
	if pelRatioStr := c.Get("REDIS_STREAMS_PEL_RATIO"); pelRatioStr != "" {
		if pelRatio, err := strconv.ParseFloat(pelRatioStr, 64); err == nil {
			// Validate range: 0.0 to 1.0
			if pelRatio >= 0.0 && pelRatio <= 1.0 {
				streamsConfig.PELRatio = pelRatio
			}
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
