package scylladb

import (
	"github.com/gocql/gocql"
	"go.opentelemetry.io/otel/trace"
)

// Config contains all the necessary configurations to connect to a scylladb cluster.
type Config struct {
	Hosts    string
	Username string
	Password string
	Keyspace string
	Port     int
}

// Client represents a scylladb client.
type Client struct {
	session *gocql.Session
	logger  Logger
	metrics Metrics
	config  Config
	tracer  trace.Tracer
}

// newClusterConfig creates some basic configurations for the scylladb cluster.
func newClusterConfig(config Config, rPolicy gocql.RetryPolicy) *gocql.ClusterConfig {
	var retryPolicy gocql.RetryPolicy
	if rPolicy == nil {
		retryPolicy = &gocql.SimpleRetryPolicy{
			NumRetries: 3,
		}
	}

	c := gocql.NewCluster(config.Hosts)
	c.Keyspace = config.Keyspace
	c.Port = config.Port
	c.RetryPolicy = retryPolicy
	c.Authenticator = gocql.PasswordAuthenticator{
		Username: config.Username,
		Password: config.Password,
	}

	return c
}

// UseTracer sets the tracer to be used by the client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// New creates a new scylladb client.
func New(cfg Config) (*Client, error) {
	cluster := newClusterConfig(cfg, nil)

	session, err := cluster.CreateSession()

	if err != nil {
		return nil, err
	}

	return &Client{session: session}, nil
}

func (c *Client) Connect() {
	c.logger.Debug("connecting to scylladb")

	scyllaBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10, 15, 20, 30, 50, 75, 100}
	c.metrics.NewHistogram("app_scylladb_stats", "Response time of SCYLLADB queries in microseconds.", scyllaBuckets...)

	c.logger.Logf("connected to scylladb at host '%s'", c.config.Hosts)

}

func (c *Client) HealthCheck() error {
	return nil
}
