package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	statusDown     = "DOWN"
	statusUp       = "UP"
	defaultTimeout = 5 * time.Second
)

var (
	errEmptyIndex        = errors.New("index name cannot be empty")
	errEmptyDocumentID   = errors.New("document ID cannot be empty")
	errEmptyQuery        = errors.New("query cannot be empty")
	errEmptyOperations   = errors.New("operations cannot be empty")
	errHealthCheckFailed = errors.New("elasticsearch health check failed")
	errOperation         = errors.New("elasticsearch operation error")
	errMarshaling        = errors.New("error marshaling data")
	errParsingResponse   = errors.New("error parsing response")
	errResponse          = errors.New("invalid elasticsearch response")
	errEncodingOperation = errors.New("error encoding operation")
)

// Config holds the configuration for connecting to Elasticsearch.
type Config struct {
	Addresses []string
	Username  string
	Password  string
}

// Client represents the Elasticsearch client.
type Client struct {
	config  Config
	client  *es.Client
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

// UseLogger sets the logger for the Elasticsearch client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Elasticsearch client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for Elasticsearch client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// New creates a new Elasticsearch client with the provided configuration.
func New(config Config) *Client {
	return &Client{
		config: config,
	}
}

func (c *Client) Connect() {
	cfg := es.Config{
		Addresses: c.config.Addresses,
		Username:  c.config.Username,
		Password:  c.config.Password,
	}

	c.logger.Debugf("connecting to Elasticsearch at %v", c.config.Addresses)

	client, err := es.NewClient(cfg)
	if err != nil {
		c.logger.Errorf("error creating Elasticsearch client: %v", err)

		return
	}

	c.client = client

	var responseTimeBuckets = []float64{0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0, 10.0}

	c.metrics.NewHistogram("es_request_duration_ms", "Duration of Elasticsearch requests in ms", responseTimeBuckets...)

	if _, err := c.HealthCheck(context.Background()); err != nil {
		c.logger.Errorf("Elasticsearch health check failed: %v", err)
		return
	}

	c.logger.Logf("connected to Elasticsearch successfully at : %v", c.config.Addresses)
}

// CreateIndex creates an index in Elasticsearch with the specified settings.
func (c *Client) CreateIndex(ctx context.Context, index string, settings map[string]any) error {
	if strings.TrimSpace(index) == "" {
		return errEmptyIndex
	}

	start := time.Now()

	tracedCtx, span := c.addTrace(ctx, "create-index", []string{index}, "")

	body, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("%w: settings: %w", errMarshaling, err)
	}

	req := esapi.IndicesCreateRequest{
		Index: index,
		Body:  bytes.NewReader(body),
	}

	res, err := req.Do(tracedCtx, c.client)
	if err != nil {
		return fmt.Errorf("%w: creating index: %w", errOperation, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%w: %s", errResponse, res.String())
	}

	c.sendOperationStats(start, fmt.Sprintf("CREATE INDEX %s", index),
		[]string{index}, "", settings, span)

	return nil
}

func (c *Client) DeleteIndex(ctx context.Context, index string) error {
	if strings.TrimSpace(index) == "" {
		return errEmptyIndex
	}

	start := time.Now()

	tracedCtx, span := c.addTrace(ctx, "delete-index", []string{index}, "")

	req := esapi.IndicesDeleteRequest{
		Index: []string{index},
	}

	res, err := req.Do(tracedCtx, c.client)
	if err != nil {
		return fmt.Errorf("%w: deleting index: %w", errOperation, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%w: %s", errResponse, res.String())
	}

	c.sendOperationStats(start, fmt.Sprintf("DELETE INDEX %s", index),
		[]string{index}, "", nil, span)

	return nil
}

// Search executes a query against one or more indices.
// Returns the entire response JSON as a map.
func (c *Client) Search(ctx context.Context, indices []string, query map[string]any) (map[string]any, error) {
	if len(indices) == 0 {
		return nil, errEmptyIndex
	}

	if len(query) == 0 {
		return nil, errEmptyQuery
	}

	start := time.Now()

	tracedCtx, span := c.addTrace(ctx, "search", indices, "")

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("%w: query: %w", errMarshaling, err)
	}

	req := esapi.SearchRequest{
		Index: indices,
		Body:  bytes.NewReader(body),
	}

	res, err := req.Do(tracedCtx, c.client)
	if err != nil {
		return nil, fmt.Errorf("%w: executing search: %w", errOperation, err)
	}

	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("%w: %s", errResponse, res.String())
	}

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", errParsingResponse, err)
	}

	c.sendOperationStats(start, "SEARCH", indices, "", query, span)

	return result, nil
}

// Bulk executes multiple indexing/updating/deleting operations in one request.
// Each entry in `operations` should be a JSON‑serializable object
// following the Elasticsearch bulk API format.
func (c *Client) Bulk(ctx context.Context, operations []map[string]any) (map[string]any, error) {
	if len(operations) == 0 {
		return nil, errEmptyOperations
	}

	start := time.Now()
	tracedCtx, span := c.addTrace(ctx, "bulk", nil, "")

	var buf bytes.Buffer
	for _, op := range operations {
		if err := json.NewEncoder(&buf).Encode(op); err != nil {
			return nil, fmt.Errorf("%w: %w", errEncodingOperation, err)
		}
	}

	req := esapi.BulkRequest{
		Body:    &buf,
		Refresh: "true",
	}

	res, err := req.Do(tracedCtx, c.client)
	if err != nil {
		return nil, fmt.Errorf("%w: executing bulk: %w", errOperation, err)
	}

	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("%w: %s", errResponse, res.String())
	}

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", errParsingResponse, err)
	}

	c.sendOperationStats(start, "BULK", nil, "", operations, span)

	return result, nil
}

// Health represents the health status of Elasticsearch connection.
type Health struct {
	Status  string         `json:"status"`            // "UP" or "DOWN"
	Details map[string]any `json:"details,omitempty"` // extra metadata
}

// HealthCheck verifies connectivity via Ping, then enriches via Info().
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{Details: make(map[string]any)}
	h.Details["addresses"] = c.config.Addresses
	h.Details["username"] = c.config.Username

	// 1) Ping with a 2s timeout
	pingCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	pingRes, err := c.client.Ping(
		c.client.Ping.WithContext(pingCtx),
	)
	if err != nil {
		h.Status = statusDown
		h.Details["error"] = err.Error()

		return &h, errHealthCheckFailed
	}
	defer pingRes.Body.Close()

	if pingRes.IsError() {
		h.Status = statusDown
		h.Details["error"] = pingRes.String()

		return &h, errHealthCheckFailed
	}

	// 2) Fetch cluster info for more details
	infoRes, err := c.client.Info()
	if err == nil {
		defer infoRes.Body.Close()

		var clusterInfo struct {
			ClusterName string `json:"cluster_name"`
			Version     struct {
				Number string `json:"number"`
			} `json:"version"`
		}

		if err := json.NewDecoder(infoRes.Body).Decode(&clusterInfo); err == nil {
			h.Details["cluster_name"] = clusterInfo.ClusterName
			h.Details["version"] = clusterInfo.Version.Number
		}
	}

	h.Status = statusUp

	return &h, nil
}

func (c *Client) addTrace(ctx context.Context, method string, indices []string,
	documentID string) (context.Context, trace.Span) {
	if c.tracer == nil {
		return ctx, nil
	}

	spanName := fmt.Sprintf("elasticsearch-%s", strings.ToLower(method))
	tracedCtx, span := c.tracer.Start(ctx, spanName)

	span.SetAttributes(attribute.String("db.operation", method))

	if len(indices) > 0 {
		span.SetAttributes(attribute.StringSlice("elasticsearch.indices", indices))
	}

	if documentID != "" {
		span.SetAttributes(attribute.String("elasticsearch.document_id", documentID))
	}

	return tracedCtx, span
}

func (c *Client) sendOperationStats(start time.Time,
	operation string, indices []string, documentID string, request any, span trace.Span) {
	duration := time.Since(start).Microseconds()

	if span != nil {
		span.SetAttributes(
			attribute.Int64("elasticsearch.duration", duration),
		)

		if request != nil {
			if b, err := json.Marshal(request); err == nil {
				span.SetAttributes(attribute.String("elasticsearch.query", clean(string(b))))
			}
		}

		span.End()
	}

	c.metrics.RecordHistogram(context.Background(), "es_request_duration_ms", float64(duration))

	// Structured log
	ql := QueryLog{
		Operation:  operation,
		Indices:    indices,
		DocumentID: documentID,
		Request:    request,
		Duration:   duration,
	}

	ql.PrettyPrint(c.logger)
}
