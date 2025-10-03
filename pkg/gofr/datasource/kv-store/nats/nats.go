package nats

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	errStatusDown  = errors.New("status down")
	errKeyNotFound = errors.New("key not found")
)

type Configs struct {
	Server string
	Bucket string
}

type jetStream struct {
	nats.JetStreamContext
}

func (j jetStream) AccountInfo() (*nats.AccountInfo, error) {
	return j.JetStreamContext.AccountInfo()
}

type Client struct {
	conn    *nats.Conn
	js      JetStream
	kv      nats.KeyValue
	configs *Configs
	tracer  trace.Tracer
	metrics Metrics
	logger  Logger
}

// New creates a new NATS-KV client with the provided configuration.
func New(configs Configs) *Client {
	return &Client{configs: &configs}
}

// UseLogger sets the logger for the NATS-KV client which asserts the Logger interface.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the NATS-KV client which asserts the Metrics interface.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for NATS-KV client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to NATS-KV and registers metrics using the provided configuration when the client is created.
func (c *Client) Connect() {
	c.logger.Debugf("connecting to NATS-KV Store at %v with bucket %q", c.configs.Server, c.configs.Bucket)

	natsBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_nats_kv_stats", "Response time of NATS KV operations in milliseconds.", natsBuckets...)

	nc, err := nats.Connect(c.configs.Server)
	if err != nil {
		c.logger.Errorf("error while connecting to NATS: %v", err)
		return
	}

	c.conn = nc
	c.logger.Debug("connection to NATS successful")

	js, err := nc.JetStream()
	if err != nil {
		c.logger.Errorf("error while initializing JetStream: %v", err)
		return
	}

	c.js = jetStream{js}

	c.logger.Debug("jetStream initialized successfully")

	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: c.configs.Bucket,
	})
	if err != nil {
		c.logger.Errorf("error while creating/accessing KV bucket: %v", err)
		return
	}

	c.kv = kv
	c.logger.Infof("successfully connected to NATS-KV Store at %s:%s ", c.configs.Server, c.configs.Bucket)
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	span := c.addTrace(ctx, "get", key)
	defer c.sendOperationStats(time.Now(), "GET", "get", span, key)

	entry, err := c.kv.Get(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return "", fmt.Errorf("%w: %s", errKeyNotFound, key)
		}

		return "", fmt.Errorf("failed to get key: %w", err)
	}

	return string(entry.Value()), nil
}

func (c *Client) Set(ctx context.Context, key, value string) error {
	span := c.addTrace(ctx, "set", key)
	defer c.sendOperationStats(time.Now(), "SET", "set", span, key, value)

	_, err := c.kv.Put(key, []byte(value))
	if err != nil {
		return fmt.Errorf("failed to set key-value pair: %w", err)
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	span := c.addTrace(ctx, "delete", key)
	defer c.sendOperationStats(time.Now(), "DELETE", "delete", span, key)

	err := c.kv.Delete(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return fmt.Errorf("%w: %s", errKeyNotFound, key)
		}

		return fmt.Errorf("failed to delete key: %w", err)
	}

	return nil
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	start := time.Now()
	span := c.addTrace(ctx, "healthcheck", c.configs.Bucket)

	h := &Health{
		Details: make(map[string]any),
	}

	h.Details["url"] = c.configs.Server
	h.Details["bucket"] = c.configs.Bucket

	_, err := c.js.AccountInfo()
	if err != nil {
		h.Status = "DOWN"

		c.logger.Debug(&Log{
			Type:     "HEALTH CHECK",
			Key:      "health",
			Value:    fmt.Sprintf("Connection failed for bucket '%s' at '%s'", c.configs.Bucket, c.configs.Server),
			Duration: time.Since(start).Microseconds(),
		})

		if span != nil {
			span.End()
		}

		return h, errStatusDown
	}

	h.Status = "UP"

	c.logger.Debug(&Log{
		Type:     "HEALTH CHECK",
		Key:      "health",
		Value:    fmt.Sprintf("Checking connection status for bucket '%s' at '%s'", c.configs.Bucket, c.configs.Server),
		Duration: time.Since(start).Microseconds(),
	})

	if span != nil {
		span.End()
	}

	return h, nil
}

func (c *Client) sendOperationStats(start time.Time, methodType, method string, span trace.Span, kv ...string) {
	duration := time.Since(start)

	var key string
	if len(kv) > 0 {
		key = kv[0]
	}

	c.logger.Debug(&Log{
		Type:     methodType,
		Duration: duration.Microseconds(),
		Key:      key,
		Value:    c.configs.Bucket,
	})

	if span != nil {
		defer span.End()

		span.SetAttributes(attribute.Int64(fmt.Sprintf("natskv.%v.duration(μs)", method), duration.Microseconds()))
	}

	c.metrics.RecordHistogram(context.Background(), "app_nats_kv_stats", float64(duration.Milliseconds()),
		"bucket", c.configs.Bucket,
		"operation", methodType)
}

func (c *Client) addTrace(ctx context.Context, method, key string) trace.Span {
	if c.tracer != nil {
		_, span := c.tracer.Start(ctx, fmt.Sprintf("natskv-%v", method))
		span.SetAttributes(attribute.String("natskv.key", key))
		span.SetAttributes(attribute.String("operation", method))

		return span
	}

	return nil
}
