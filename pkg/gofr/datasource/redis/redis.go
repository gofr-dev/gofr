package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"time"

	otel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	redisPingTimeout = 5 * time.Second
	defaultRedisPort = 6379
)

type Config struct {
	HostName string
	Username string
	Password string
	Port     int
	DB       int
	Options  *redis.Options
	TLS      *tls.Config
}

type Redis struct {
	*redis.Client
	logger datasource.Logger
	config *Config
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
	}

	return &Redis{Client: rc, config: redisConfig, logger: logger}
}

// Close shuts down the Redis client, ensuring the current dataset is saved before exiting.
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}

	return nil
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

func initializeCerts(logger datasource.Logger, caCert []byte, tlsConfig *tls.Config) {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		logger.Errorf("failed to append CA cert to pool")
	} else {
		tlsConfig.RootCAs = caCertPool
	}
}

// TODO - if we make Redis an interface and expose from container we can avoid c.Redis(c, command) using methods on c and still pass c.
// type Redis interface {
//	Get(string) (string, error)
// }
