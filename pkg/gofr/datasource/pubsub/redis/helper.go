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
	config := &tls.Config{
		InsecureSkipVerify: tlsConf.InsecureSkipVerify,
	}

	// Load client certificate if provided
	if tlsConf.CertFile != "" && tlsConf.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsConf.CertFile, tlsConf.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		config.Certificates = []tls.Certificate{cert}
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

		config.RootCAs = caCertPool
	}

	return config, nil
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

	// Read address - prefer REDIS_PUBSUB_ADDR, fallback to REDIS_HOST:REDIS_PORT
	if addr := c.Get("REDIS_PUBSUB_ADDR"); addr != "" {
		cfg.Addr = addr
	} else if host := c.Get("REDIS_HOST"); host != "" {
		port := c.GetOrDefault("REDIS_PORT", "6379")
		cfg.Addr = fmt.Sprintf("%s:%s", host, port)
	}

	// Read password
	cfg.Password = c.Get("REDIS_PASSWORD")

	// Read database - prefer REDIS_PUBSUB_DB, fallback to REDIS_DB
	if dbStr := c.Get("REDIS_PUBSUB_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil && db >= 0 {
			cfg.DB = db
		}
	} else if dbStr := c.Get("REDIS_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil && db >= 0 {
			cfg.DB = db
		}
	}

	// Parse timeouts
	if timeout := c.Get("REDIS_PUBSUB_DIAL_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil && d > 0 {
			cfg.DialTimeout = d
		}
	}

	if timeout := c.Get("REDIS_PUBSUB_READ_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil && d > 0 {
			cfg.ReadTimeout = d
		}
	}

	if timeout := c.Get("REDIS_PUBSUB_WRITE_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil && d > 0 {
			cfg.WriteTimeout = d
		}
	}

	// Parse connection pool settings
	if poolSize := c.Get("REDIS_PUBSUB_POOL_SIZE"); poolSize != "" {
		if size, err := strconv.Atoi(poolSize); err == nil && size > 0 {
			cfg.PoolSize = size
		}
	}

	if minIdle := c.Get("REDIS_PUBSUB_MIN_IDLE_CONNS"); minIdle != "" {
		if n, err := strconv.Atoi(minIdle); err == nil && n >= 0 {
			cfg.MinIdleConns = n
		}
	}

	if maxIdle := c.Get("REDIS_PUBSUB_MAX_IDLE_CONNS"); maxIdle != "" {
		if n, err := strconv.Atoi(maxIdle); err == nil && n >= 0 {
			cfg.MaxIdleConns = n
		}
	}

	// Parse max retries
	if retries := c.Get("REDIS_PUBSUB_MAX_RETRIES"); retries != "" {
		if n, err := strconv.Atoi(retries); err == nil && n >= 0 {
			cfg.MaxRetries = n
		}
	}

	// Setup TLS if enabled
	if c.Get("REDIS_TLS_ENABLED") == "true" {
		tlsConfig := &TLSConfig{
			InsecureSkipVerify: c.Get("REDIS_TLS_INSECURE_SKIP_VERIFY") == "true",
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

	return cfg
}
