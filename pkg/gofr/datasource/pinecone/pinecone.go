package pinecone

import (
	"context"
	"fmt"
	"time"

	"github.com/pinecone-io/go-pinecone/v3/pinecone"
	"go.opentelemetry.io/otel/trace"
)

const (
	// Status constants for health checks
	statusDown = "DOWN"
	statusUp   = "UP"

	// Metric constants
	metricCosine     = "cosine"
	metricEuclidean  = "euclidean"
	metricDotProduct = "dotproduct"

	// Cloud provider constants
	cloudAWS   = "aws"
	cloudGCP   = "gcp"
	cloudAzure = "azure"

	// Default values
	defaultRegion   = "us-east-1"
	maxIndexDisplay = 5

	// Metrics configuration
	metricsHistogramName = "app_pinecone_stats"
	metricsGaugeName     = "app_pinecone_operations"
	metricsDescription   = "Response time of Pinecone operations in seconds."
	gaugeDescription     = "Number of Pinecone operations."
)

// MetricsConfig holds configuration for metrics setup
type MetricsConfig struct {
	histogramBuckets []float64
}

// Health represents the health status of the Pinecone connection.
type Health struct {
	Status  string         `json:"status"`
	Details map[string]any `json:"details"`
}

// Client represents a Pinecone vector database client using the official SDK.
type Client struct {
	config    *Config
	client    *pinecone.Client
	logger    Logger
	metrics   Metrics
	tracer    trace.Tracer
	connected bool

	// Composed components
	connector     *connectionManager
	healthChecker *healthChecker
	indexManager  *indexManager
	vectorManager *vectorManager
	spanManager   *spanManager
}

// New creates a new Pinecone client with the provided configuration.
func New(config *Config) *Client {
	c := &Client{
		config: config,
	}

	c.initializeComponents()
	return c
}

// initializeComponents sets up the client's composed components
func (c *Client) initializeComponents() {
	c.connector = newConnectionManager(c)
	c.healthChecker = newHealthChecker(c)
	c.indexManager = newIndexManager(c)
	c.vectorManager = newVectorManager(c)
	c.spanManager = newSpanManager(c)
}

// UseLogger sets the logger for the Pinecone client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Pinecone client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the Pinecone client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to Pinecone using the official SDK.
func (c *Client) Connect() {
	c.connector.connect()
}

// HealthCheck performs a health check on the Pinecone connection.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	return c.healthChecker.check(ctx)
}

// ListIndexes returns all available indexes in the Pinecone project.
func (c *Client) ListIndexes(ctx context.Context) ([]string, error) {
	return c.indexManager.listIndexes(ctx)
}

// DescribeIndex retrieves detailed information about a specific index.
func (c *Client) DescribeIndex(ctx context.Context, indexName string) (map[string]any, error) {
	return c.indexManager.describeIndex(ctx, indexName)
}

// CreateIndex creates a new Pinecone index with the given parameters.
func (c *Client) CreateIndex(ctx context.Context, indexName string, dimension int, metric string, options map[string]any) error {
	return c.indexManager.createIndex(ctx, indexName, dimension, metric, options)
}

// DeleteIndex deletes a Pinecone index.
func (c *Client) DeleteIndex(ctx context.Context, indexName string) error {
	return c.indexManager.deleteIndex(ctx, indexName)
}

// Upsert adds or updates vectors in a specific namespace of an index.
func (c *Client) Upsert(ctx context.Context, indexName, namespace string, vectors []any) (int, error) {
	return c.vectorManager.upsert(ctx, indexName, namespace, vectors)
}

// Query searches for similar vectors in the index.
func (c *Client) Query(ctx context.Context, params QueryParams) ([]any, error) {
	return c.vectorManager.query(ctx, params)
}

// Fetch retrieves vectors by their IDs.
func (c *Client) Fetch(ctx context.Context, indexName, namespace string, ids []string) (map[string]any, error) {
	return c.vectorManager.fetch(ctx, indexName, namespace, ids)
}

// Delete removes vectors from the index.
func (c *Client) Delete(ctx context.Context, indexName, namespace string, ids []string) error {
	return c.vectorManager.delete(ctx, indexName, namespace, ids)
}

// DeleteAll removes all vectors from a namespace.
func (c *Client) DeleteAll(ctx context.Context, indexName, namespace string) error {
	return c.vectorManager.deleteAll(ctx, indexName, namespace)
}

// IsConnected checks if the Pinecone client is currently connected.
func (c *Client) IsConnected() bool {
	return c.connected && c.client != nil
}

// Ping performs a quick connectivity test to Pinecone.
func (c *Client) Ping(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("pinecone client not connected")
	}

	_, err := c.client.ListIndexes(ctx)
	return err
}

// getDefaultMetricsConfig returns the default metrics configuration
func getDefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		histogramBuckets: []float64{.05, .1, .25, .5, .75, 1, 1.5, 2, 3, 5, 7.5, 10, 15, 30},
	}
}

// recordMetrics records the duration of a Pinecone operation.
func (c *Client) recordMetrics(start time.Time, operation string) {
	if c.metrics != nil {
		duration := time.Since(start).Seconds()
		c.metrics.RecordHistogram(context.Background(), "app_pinecone_stats", duration,
			"operation", operation)
	}
}

// validateConnection checks if the client is connected
func (c *Client) validateConnection() error {
	if !c.connected || c.client == nil {
		return fmt.Errorf("pinecone client not connected")
	}
	return nil
}


