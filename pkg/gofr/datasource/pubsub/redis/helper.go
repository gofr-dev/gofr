package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr/config"
)

const (
	defaultRetryTimeout  = 10 * time.Second
	messageBufferSize    = 100
	unsubscribeOpTimeout = 2 * time.Second
	goroutineWaitTimeout = 5 * time.Second
)

// validateConfigs validates the Redis configuration.
func validateConfigs(conf *Config) error {
	if conf.Addr == "" {
		return errAddrNotProvided
	}

	if conf.DB < 0 {
		return fmt.Errorf("%w: %d", errInvalidDB, conf.DB)
	}

	if conf.PoolSize <= 0 {
		conf.PoolSize = 10
	}

	if conf.DialTimeout <= 0 {
		conf.DialTimeout = 5 * time.Second
	}

	if conf.ReadTimeout <= 0 {
		conf.ReadTimeout = 3 * time.Second
	}

	if conf.WriteTimeout <= 0 {
		conf.WriteTimeout = 3 * time.Second
	}

	return nil
}

// createRedisOptions creates redis.Options from Config.
func createRedisOptions(conf *Config) (*redis.Options, error) {
	options := &redis.Options{
		Addr:         conf.Addr,
		Password:     conf.Password,
		DB:           conf.DB,
		MaxRetries:   conf.MaxRetries,
		DialTimeout:  conf.DialTimeout,
		ReadTimeout:  conf.ReadTimeout,
		WriteTimeout: conf.WriteTimeout,
		PoolSize:     conf.PoolSize,
		MinIdleConns: conf.MinIdleConns,
		MaxIdleConns: conf.MaxIdleConns,
	}

	if conf.ConnMaxIdleTime > 0 {
		options.ConnMaxIdleTime = conf.ConnMaxIdleTime
	}

	if conf.ConnMaxLifetime > 0 {
		options.ConnMaxLifetime = conf.ConnMaxLifetime
	}

	// Setup TLS if configured
	if conf.TLS != nil {
		tlsConfig, err := createTLSConfig(conf.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}

		options.TLSConfig = tlsConfig
	}

	return options, nil
}

// createTLSConfig creates a TLS configuration from TLSConfig.
//
//nolint:gosec // InsecureSkipVerify may be set by user for testing/development
func createTLSConfig(tlsConf *TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: tlsConf.InsecureSkipVerify,
	}

	// Load client certificate if provided
	if tlsConf.CertFile != "" && tlsConf.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsConf.CertFile, tlsConf.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if provided
	if tlsConf.CACertFile != "" {
		caCert, err := os.ReadFile(tlsConf.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()

		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errFailedToParseCACert
		}

		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// isConnected checks if the Redis client is connected.
func (r *Client) isConnected() bool {
	if r.pubConn == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRetryTimeout)
	defer cancel()

	err := r.pubConn.Ping(ctx).Err()

	return err == nil
}

// parseQueryArgs parses query arguments for Query method.
func parseQueryArgs(args ...any) (timeout time.Duration, limit int) {
	timeout = 30 * time.Second
	limit = 10

	if len(args) > 0 {
		if val, ok := args[0].(time.Duration); ok && val > 0 {
			timeout = val
		}
	}

	if len(args) > 1 {
		if val, ok := args[1].(int); ok && val > 0 {
			limit = val
		}
	}

	return timeout, limit
}

// getRedisPubSubConfig builds Config from config.Config (reads env vars).
// Similar to getRedisConfig() in datasource/redis/redis.go.
// It supports the following environment variables:
//
//	REDIS_PUBSUB_ADDR: Redis address (e.g., "localhost:6379") - primary config
//	REDIS_HOST: Redis host (fallback if REDIS_PUBSUB_ADDR not set)
//	REDIS_PORT: Redis port (fallback, default: 6379)
//	REDIS_PUBSUB_DB: Redis database number for PubSub (fallback to REDIS_DB)
//	REDIS_DB: Redis database (fallback, default: 0)
//	REDIS_PASSWORD: Redis password
//	REDIS_PUBSUB_DIAL_TIMEOUT: Connection timeout (default: 5s)
//	REDIS_PUBSUB_READ_TIMEOUT: Read timeout (default: 3s)
//	REDIS_PUBSUB_WRITE_TIMEOUT: Write timeout (default: 3s)
//	REDIS_TLS_ENABLED: Enable TLS (set to "true")
//	REDIS_TLS_CA_CERT: CA certificate file path
//	REDIS_TLS_CERT: Client certificate file path
//	REDIS_TLS_KEY: Client key file path
//	REDIS_TLS_INSECURE_SKIP_VERIFY: Skip TLS verification (set to "true")
func getRedisPubSubConfig(c config.Config) *Config {
	cfg := DefaultConfig()

	parseAddress(c, cfg)
	cfg.Password = c.Get("REDIS_PASSWORD")
	parseDatabase(c, cfg)
	parseTimeouts(c, cfg)
	parsePoolSettings(c, cfg)
	parseMaxRetries(c, cfg)
	parseTLSConfig(c, cfg)

	return cfg
}

// parseAddress parses the Redis address from config.
func parseAddress(c config.Config, cfg *Config) {
	if addr := c.Get("REDIS_PUBSUB_ADDR"); addr != "" {
		cfg.Addr = addr

		return
	}

	if host := c.Get("REDIS_HOST"); host != "" {
		port := c.GetOrDefault("REDIS_PORT", "6379")
		cfg.Addr = fmt.Sprintf("%s:%s", host, port)
	}
}

// parseDatabase parses the database number from config.
func parseDatabase(c config.Config, cfg *Config) {
	if dbStr := c.Get("REDIS_PUBSUB_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil && db >= 0 {
			cfg.DB = db
		}

		return
	}

	if dbStr := c.Get("REDIS_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil && db >= 0 {
			cfg.DB = db
		}
	}
}

// parseTimeouts parses timeout settings from config.
func parseTimeouts(c config.Config, cfg *Config) {
	parseDuration(c, "REDIS_PUBSUB_DIAL_TIMEOUT", func(d time.Duration) { cfg.DialTimeout = d })
	parseDuration(c, "REDIS_PUBSUB_READ_TIMEOUT", func(d time.Duration) { cfg.ReadTimeout = d })
	parseDuration(c, "REDIS_PUBSUB_WRITE_TIMEOUT", func(d time.Duration) { cfg.WriteTimeout = d })
}

// parsePoolSettings parses connection pool settings from config.
func parsePoolSettings(c config.Config, cfg *Config) {
	parseInt(c, "REDIS_PUBSUB_POOL_SIZE", func(n int) { cfg.PoolSize = n }, func(n int) bool { return n > 0 })
	parseInt(c, "REDIS_PUBSUB_MIN_IDLE_CONNS", func(n int) { cfg.MinIdleConns = n }, func(n int) bool { return n >= 0 })
	parseInt(c, "REDIS_PUBSUB_MAX_IDLE_CONNS", func(n int) { cfg.MaxIdleConns = n }, func(n int) bool { return n >= 0 })
}

// parseDuration parses a duration from config and applies it if valid.
func parseDuration(c config.Config, key string, setter func(time.Duration)) {
	if timeout := c.Get(key); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil && d > 0 {
			setter(d)
		}
	}
}

// parseInt parses an integer from config and applies it if valid.
func parseInt(c config.Config, key string, setter func(int), validator func(int) bool) {
	if val := c.Get(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil && validator(n) {
			setter(n)
		}
	}
}

// parseMaxRetries parses max retries setting from config.
func parseMaxRetries(c config.Config, cfg *Config) {
	if retries := c.Get("REDIS_PUBSUB_MAX_RETRIES"); retries != "" {
		if n, err := strconv.Atoi(retries); err == nil && n >= 0 {
			cfg.MaxRetries = n
		}
	}
}

// parseTLSConfig parses TLS configuration from config.
func parseTLSConfig(c config.Config, cfg *Config) {
	const tlsEnabled = "true"

	if c.Get("REDIS_TLS_ENABLED") != tlsEnabled {
		return
	}

	tlsConfig := &TLSConfig{
		InsecureSkipVerify: c.Get("REDIS_TLS_INSECURE_SKIP_VERIFY") == tlsEnabled,
	}

	if caCert := c.Get("REDIS_TLS_CA_CERT"); caCert != "" {
		tlsConfig.CACertFile = caCert
	}

	if cert := c.Get("REDIS_TLS_CERT"); cert != "" {
		tlsConfig.CertFile = cert
	}

	if key := c.Get("REDIS_TLS_KEY"); key != "" {
		tlsConfig.KeyFile = key
	}

	cfg.TLS = tlsConfig
}
