package influxdb

import (
	"context"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go.opencensus.io/trace"
	"gofr.dev/pkg/gofr/container"
)

// Config holds the configuration for connecting to InfluxDB.
type Config struct {
	Url      string
	Token    string
	Username string
	Password string
}

// Client represents the InfluxDB client.
type Client struct {
	config  Config
	client  influxdb2.Client
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

// CreateBucket implements container.InfluxDBProvider.
func (c *Client) CreateBucket(ctx context.Context, org string, bucket string, retentionPeriod time.Duration) error {
	panic("unimplemented")
}

// DeleteBucket implements container.InfluxDBProvider.
func (c *Client) DeleteBucket(ctx context.Context, org string, bucket string) error {
	panic("unimplemented")
}

// HealthCheck implements container.InfluxDBProvider.
func (c *Client) HealthCheck(context.Context) (any, error) {
	panic("unimplemented")
}

// ListBuckets implements container.InfluxDBProvider.
func (c *Client) ListBuckets(ctx context.Context, org string) ([]string, error) {
	panic("unimplemented")
}

// Ping implements container.InfluxDBProvider.
func (c *Client) Ping(ctx context.Context) error {
	panic("unimplemented")
}

// Query implements container.InfluxDBProvider.
func (c *Client) Query(ctx context.Context, org string, fluxQuery string) ([]map[string]any, error) {
	panic("unimplemented")
}

// UseLogger sets the logger for the Elasticsearch client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics implements container.InfluxDBProvider.
func (c *Client) UseMetrics(metrics any) {
	panic("unimplemented")
}

// UseTracer implements container.InfluxDBProvider.
func (c *Client) UseTracer(tracer any) {
	panic("unimplemented")
}

// WritePoints implements container.InfluxDBProvider.
func (c *Client) WritePoints(ctx context.Context, bucket string, org string, points []container.InfluxPoint) error {
	panic("unimplemented")
}

// New creates a new InfluxDB client with the provided configuration.
func New(config Config) *Client {
	return &Client{
		config: config,
	}
}

func (c *Client) Connect() {

	c.logger.Debugf("connecting to influxdb at %v", c.config.Url)

	// Create a new client using an InfluxDB server base URL and an authentication token
	client := influxdb2.NewClient(
		c.config.Url,
		c.config.Token,
	)
	c.client = client
	c.logger.Logf("connected to influxdb successfully at : %v", c.config.Url)
}
