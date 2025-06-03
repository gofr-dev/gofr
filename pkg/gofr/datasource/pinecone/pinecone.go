package pinecone

import (
	"context"
	"fmt"
	"time"

	"github.com/pinecone-io/go-pinecone/v3/pinecone"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
	"google.golang.org/protobuf/types/known/structpb"
)

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
}

// New creates a new Pinecone client with the provided configuration.
func New(config *Config) *Client {
	return &Client{
		config: config,
	}
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
	if c.logger != nil {
		c.logger.Debugf("connecting to Pinecone with API key")
	}

	// Check if API key is provided
	if c.config.APIKey == "" {
		if c.logger != nil {
			c.logger.Errorf("API key is required for Pinecone connection")
		}
		return
	}

	// Define histogram buckets for Pinecone operation metrics
	pineconeBuckets := []float64{.05, .1, .25, .5, .75, 1, 1.5, 2, 3, 5, 7.5, 10, 15, 30}

	// Register metrics if available
	if c.metrics != nil {
		c.metrics.NewHistogram("app_pinecone_stats", "Response time of Pinecone operations in seconds.", pineconeBuckets...)
		c.metrics.NewGauge("app_pinecone_operations", "Number of Pinecone operations.")
	}

	// Create the official Pinecone client
	client, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: c.config.APIKey,
	})

	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("failed to create Pinecone client: %v", err)
		}
		return
	}

	c.client = client
	c.connected = true

	// Log successful connection
	if c.logger != nil {
		c.logger.Infof("connected to Pinecone successfully")
	}
}

// HealthCheck performs a health check on the Pinecone connection.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	const (
		statusDown = "DOWN"
		statusUp   = "UP"
	)

	ctx, span := c.startSpan(ctx, "HealthCheck")
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "health_check")

	h := Health{
		Details: make(map[string]any),
	}

	// Add basic connection details
	h.Details["api_key_configured"] = c.config.APIKey != ""

	// Check if client is connected
	if !c.connected || c.client == nil {
		h.Status = statusDown
		h.Details["error"] = "pinecone client not connected"
		h.Details["connection_state"] = "disconnected"

		return h, fmt.Errorf("pinecone client not connected")
	}

	// Perform actual connectivity test by listing indexes
	// This is a lightweight operation that verifies API connectivity
	indexes, err := c.client.ListIndexes(ctx)
	if err != nil {
		h.Status = statusDown
		h.Details["error"] = fmt.Sprintf("failed to connect to Pinecone API: %v", err)
		h.Details["connection_state"] = "error"

		return h, err
	}

	// Connection is healthy
	h.Status = statusUp
	h.Details["index_count"] = len(indexes)
	h.Details["connection_state"] = "connected"

	// Add index names for debugging (limited to first 5 to avoid large responses)
	if len(indexes) > 0 {
		indexNames := make([]string, 0, min(len(indexes), 5))
		for i, index := range indexes {
			if i >= 5 {
				break
			}
			indexNames = append(indexNames, index.Name)
		}
		h.Details["available_indexes"] = indexNames
		if len(indexes) > 5 {
			h.Details["total_indexes"] = len(indexes)
		}
	}

	return h, nil
}

// ListIndexes returns all available indexes in the Pinecone project.
func (c *Client) ListIndexes(ctx context.Context) ([]string, error) {
	ctx, span := c.startSpan(ctx, "ListIndexes")
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "list_indexes")

	if !c.connected || c.client == nil {
		return nil, fmt.Errorf("pinecone client not connected")
	}

	indexes, err := c.client.ListIndexes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	var indexNames []string
	for _, index := range indexes {
		indexNames = append(indexNames, index.Name)
	}

	return indexNames, nil
}

// DescribeIndex retrieves detailed information about a specific index.
func (c *Client) DescribeIndex(ctx context.Context, indexName string) (map[string]any, error) {
	ctx, span := c.startSpan(ctx, "DescribeIndex")
	span.SetAttributes(attribute.String("index", indexName))
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "describe_index")

	if !c.connected || c.client == nil {
		return nil, fmt.Errorf("pinecone client not connected")
	}

	index, err := c.client.DescribeIndex(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe index %s: %w", indexName, err)
	}

	// Convert the index description to a map for compatibility
	result := map[string]any{
		"name":      index.Name,
		"dimension": index.Dimension,
		"metric":    index.Metric,
		"host":      index.Host,
		"spec":      index.Spec,
		"status":    index.Status,
	}

	return result, nil
}

// CreateIndex creates a new Pinecone index with the given parameters.
func (c *Client) CreateIndex(ctx context.Context, indexName string, dimension int, metric string, options map[string]any) error {
	ctx, span := c.startSpan(ctx, "CreateIndex")
	span.SetAttributes(
		attribute.String("index", indexName),
		attribute.Int("dimension", dimension),
		attribute.String("metric", metric),
	)
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "create_index")

	if !c.connected || c.client == nil {
		return fmt.Errorf("pinecone client not connected")
	}

	// Convert metric string to the SDK type
	var indexMetric pinecone.IndexMetric
	switch metric {
	case "cosine":
		indexMetric = pinecone.Cosine
	case "euclidean":
		indexMetric = pinecone.Euclidean
	case "dotproduct":
		indexMetric = pinecone.Dotproduct
	default:
		return fmt.Errorf("unsupported metric: %s", metric)
	}

	// Default to serverless index
	cloud := pinecone.Aws
	region := "us-east-1"

	// Apply any additional options
	if cloudStr, ok := options["cloud"].(string); ok {
		switch cloudStr {
		case "aws":
			cloud = pinecone.Aws
		case "gcp":
			cloud = pinecone.Gcp
		case "azure":
			cloud = pinecone.Azure
		}
	}

	if regionStr, ok := options["region"].(string); ok {
		region = regionStr
	}

	// Create serverless index request
	dimension32 := int32(dimension)
	req := &pinecone.CreateServerlessIndexRequest{
		Name:      indexName,
		Dimension: &dimension32,
		Metric:    &indexMetric,
		Cloud:     cloud,
		Region:    region,
	}

	_, err := c.client.CreateServerlessIndex(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create index %s: %w", indexName, err)
	}

	return nil
}

// DeleteIndex deletes a Pinecone index.
func (c *Client) DeleteIndex(ctx context.Context, indexName string) error {
	ctx, span := c.startSpan(ctx, "DeleteIndex")
	span.SetAttributes(attribute.String("index", indexName))
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "delete_index")

	if !c.connected || c.client == nil {
		return fmt.Errorf("pinecone client not connected")
	}

	err := c.client.DeleteIndex(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to delete index %s: %w", indexName, err)
	}

	return nil
}

// Upsert adds or updates vectors in a specific namespace of an index.
func (c *Client) Upsert(ctx context.Context, indexName, namespace string, vectors []any) (int, error) {
	ctx, span := c.startSpan(ctx, "Upsert")
	span.SetAttributes(
		attribute.String("index", indexName),
		attribute.String("namespace", namespace),
		attribute.Int("vectorCount", len(vectors)),
	)
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "upsert")

	if !c.connected || c.client == nil {
		return 0, fmt.Errorf("pinecone client not connected")
	}

	// Get the index description to retrieve host
	indexDesc, err := c.client.DescribeIndex(ctx, indexName)
	if err != nil {
		return 0, fmt.Errorf("failed to describe index %s: %w", indexName, err)
	}

	// Get the index connection
	indexConn, err := c.client.Index(pinecone.NewIndexConnParams{
		Host:      indexDesc.Host,
		Namespace: namespace,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create index connection: %w", err)
	}
	defer indexConn.Close()

	// Convert interface{} vectors to SDK format
	sdkVectors := make([]*pinecone.Vector, 0, len(vectors))
	for _, v := range vectors {
		if vec, ok := v.(Vector); ok {
			sdkVector := &pinecone.Vector{
				Id:     vec.ID,
				Values: &vec.Values,
			}

			if vec.Metadata != nil {
				metadata, err := structpb.NewStruct(vec.Metadata)
				if err == nil {
					sdkVector.Metadata = metadata
				}
			}

			sdkVectors = append(sdkVectors, sdkVector)
		}
	}

	// Perform upsert
	count, err := indexConn.UpsertVectors(ctx, sdkVectors)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert vectors to index %s: %w", indexName, err)
	}

	return int(count), nil
}

// Query searches for similar vectors in the index.
func (c *Client) Query(ctx context.Context, indexName, namespace string, vector []float32, topK int, includeValues bool, includeMetadata bool, filter map[string]any) ([]any, error) {
	ctx, span := c.startSpan(ctx, "Query")
	span.SetAttributes(
		attribute.String("index", indexName),
		attribute.String("namespace", namespace),
		attribute.Int("topK", topK),
	)
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "query")

	if !c.connected || c.client == nil {
		return nil, fmt.Errorf("pinecone client not connected")
	}

	// Get the index description to retrieve host
	indexDesc, err := c.client.DescribeIndex(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe index %s: %w", indexName, err)
	}

	// Get the index connection
	indexConn, err := c.client.Index(pinecone.NewIndexConnParams{
		Host:      indexDesc.Host,
		Namespace: namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create index connection: %w", err)
	}
	defer indexConn.Close()

	// Prepare query request
	req := &pinecone.QueryByVectorValuesRequest{
		Vector:          vector,
		TopK:            uint32(topK),
		IncludeValues:   includeValues,
		IncludeMetadata: includeMetadata,
	}

	// Add filter if provided
	if filter != nil {
		filterStruct, err := structpb.NewStruct(filter)
		if err == nil {
			req.MetadataFilter = filterStruct
		}
	}

	resp, err := indexConn.QueryByVectorValues(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query index %s: %w", indexName, err)
	}

	// Convert response to interface{} slice for compatibility
	results := make([]any, 0, len(resp.Matches))
	for _, match := range resp.Matches {
		result := ScoredVector{
			ID:    match.Vector.Id,
			Score: match.Score,
		}

		if match.Vector.Values != nil {
			result.Values = *match.Vector.Values
		}

		if match.Vector.Metadata != nil {
			result.Metadata = match.Vector.Metadata.AsMap()
		}

		results = append(results, result)
	}

	return results, nil
}

// Fetch retrieves vectors by their IDs.
func (c *Client) Fetch(ctx context.Context, indexName, namespace string, ids []string) (map[string]any, error) {
	ctx, span := c.startSpan(ctx, "Fetch")
	span.SetAttributes(
		attribute.String("index", indexName),
		attribute.String("namespace", namespace),
		attribute.Int("idCount", len(ids)),
	)
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "fetch")

	if !c.connected || c.client == nil {
		return nil, fmt.Errorf("pinecone client not connected")
	}

	// Get the index description to retrieve host
	indexDesc, err := c.client.DescribeIndex(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe index %s: %w", indexName, err)
	}

	// Get the index connection
	indexConn, err := c.client.Index(pinecone.NewIndexConnParams{
		Host:      indexDesc.Host,
		Namespace: namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create index connection: %w", err)
	}
	defer indexConn.Close()

	resp, err := indexConn.FetchVectors(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vectors from index %s: %w", indexName, err)
	}

	// Convert response to map[string]interface{} for compatibility
	results := make(map[string]any)
	for id, vector := range resp.Vectors {
		vec := Vector{
			ID: vector.Id,
		}

		if vector.Values != nil {
			vec.Values = *vector.Values
		}

		if vector.Metadata != nil {
			vec.Metadata = vector.Metadata.AsMap()
		}

		results[id] = vec
	}

	return results, nil
}

// Delete removes vectors from the index.
func (c *Client) Delete(ctx context.Context, indexName, namespace string, ids []string) error {
	ctx, span := c.startSpan(ctx, "Delete")
	span.SetAttributes(
		attribute.String("index", indexName),
		attribute.String("namespace", namespace),
		attribute.Int("idCount", len(ids)),
	)
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "delete")

	if !c.connected || c.client == nil {
		return fmt.Errorf("pinecone client not connected")
	}

	// Get the index description to retrieve host
	indexDesc, err := c.client.DescribeIndex(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to describe index %s: %w", indexName, err)
	}

	// Get the index connection
	indexConn, err := c.client.Index(pinecone.NewIndexConnParams{
		Host:      indexDesc.Host,
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("failed to create index connection: %w", err)
	}
	defer indexConn.Close()

	err = indexConn.DeleteVectorsById(ctx, ids)
	if err != nil {
		return fmt.Errorf("failed to delete vectors from index %s: %w", indexName, err)
	}

	return nil
}

// DeleteAll removes all vectors from a namespace.
func (c *Client) DeleteAll(ctx context.Context, indexName, namespace string) error {
	ctx, span := c.startSpan(ctx, "DeleteAll")
	span.SetAttributes(
		attribute.String("index", indexName),
		attribute.String("namespace", namespace),
	)
	defer span.End()

	start := time.Now()
	defer c.recordMetrics(start, "delete_all")

	if !c.connected || c.client == nil {
		return fmt.Errorf("pinecone client not connected")
	}

	// Get the index description to retrieve host
	indexDesc, err := c.client.DescribeIndex(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to describe index %s: %w", indexName, err)
	}

	// Get the index connection
	indexConn, err := c.client.Index(pinecone.NewIndexConnParams{
		Host:      indexDesc.Host,
		Namespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("failed to create index connection: %w", err)
	}
	defer indexConn.Close()

	err = indexConn.DeleteAllVectorsInNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete all vectors from index %s namespace %s: %w", indexName, namespace, err)
	}

	return nil
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

	// Use ListIndexes as a lightweight ping operation
	_, err := c.client.ListIndexes(ctx)
	return err
}

// startSpan starts a new trace span with the given name.
func (c *Client) startSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	if c.tracer != nil {
		return c.tracer.Start(ctx, fmt.Sprintf("pinecone.%s", name))
	}

	return ctx, noopSpan{}
}

// recordMetrics records the duration of a Pinecone operation.
func (c *Client) recordMetrics(start time.Time, operation string) {
	if c.metrics != nil {
		duration := time.Since(start).Seconds()
		c.metrics.RecordHistogram(context.Background(), "app_pinecone_stats", duration,
			"operation", operation)
	}
}

// noopSpan implements a no-op span for tracing
type noopSpan struct {
	embedded.Span
}

func (noopSpan) End(...trace.SpanEndOption)              {}
func (noopSpan) AddEvent(string, ...trace.EventOption)   {}
func (noopSpan) IsRecording() bool                       { return false }
func (noopSpan) RecordError(error, ...trace.EventOption) {}
func (noopSpan) SpanContext() trace.SpanContext          { return trace.SpanContext{} }
func (noopSpan) SetStatus(codes.Code, string)            {}
func (noopSpan) SetName(string)                          {}
func (noopSpan) SetAttributes(...attribute.KeyValue)     {}
func (noopSpan) AddLink(trace.Link)                      {}
func (noopSpan) TracerProvider() trace.TracerProvider    { return nil }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
