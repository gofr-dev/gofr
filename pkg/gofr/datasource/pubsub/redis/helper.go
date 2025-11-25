package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
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
