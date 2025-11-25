package redis

import "time"

// Config holds the configuration for Redis PubSub client.
type Config struct {
	// Addr is the Redis server address (e.g., "localhost:6379")
	Addr string

	// Password is the Redis password (optional)
	Password string

	// DB is the database number (default: 0)
	DB int

	// TLS configuration for secure connections
	TLS *TLSConfig

	// MaxRetries is the maximum number of retries for failed commands
	MaxRetries int

	// DialTimeout is the timeout for establishing connections
	DialTimeout time.Duration

	// ReadTimeout is the timeout for socket reads
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for socket writes
	WriteTimeout time.Duration

	// PoolSize is the maximum number of socket connections
	PoolSize int

	// MinIdleConns is the minimum number of idle connections
	MinIdleConns int

	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int

	// ConnMaxIdleTime is the maximum amount of time a connection may be idle
	ConnMaxIdleTime time.Duration

	// ConnMaxLifetime is the maximum amount of time a connection may be reused
	ConnMaxLifetime time.Duration
}

// TLSConfig holds TLS configuration for Redis connections.
type TLSConfig struct {
	CertFile           string
	KeyFile            string
	CACertFile         string
	InsecureSkipVerify bool
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Addr:            "localhost:6379",
		DB:              0,
		MaxRetries:      3,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        10,
		MinIdleConns:    5,
		MaxIdleConns:    10,
		ConnMaxIdleTime: 5 * time.Minute,
		ConnMaxLifetime: 30 * time.Minute,
	}
}
