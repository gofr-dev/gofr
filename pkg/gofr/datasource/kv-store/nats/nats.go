package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var errStatusDown = errors.New("status down")
var errKeyNotFound = errors.New("key not found")

type Configs struct {
	Server string
	Bucket string
}

type Client struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	kv      nats.KeyValue
	configs *Configs
	tracer  trace.Tracer
	metrics Metrics
	logger  Logger
}

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

// Connect establishes a connection to NATS-KV and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	c.logger.Debugf("connecting to NATS at %v with bucket %v", c.configs.Server, c.configs.Bucket)

	natsBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_natskv_stats", "Response time of NATS KV operations in milliseconds.", natsBuckets...)

	nc, err := nats.Connect(c.configs.Server)
	if err != nil {
		c.logger.Errorf("error while connecting to NATS: %v", err)
		return
	}

	c.conn = nc
	c.logger.Debugf("%s:%s Successfully established NATS connection", c.configs.Server, c.configs.Bucket)

	js, err := nc.JetStream()
	if err != nil {
		c.logger.Errorf("error while initializing JetStream: %v", err)
		return
	}

	c.js = js
	c.logger.Debugf("%s:%s Successfully initialized JetStream", c.configs.Server, c.configs.Bucket)

	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: c.configs.Bucket,
	})
	if err != nil {
		c.logger.Errorf("error while creating/accessing KV bucket: %v", err)
		return
	}

	c.kv = kv
	c.logger.Infof("%s:%s Successfully connected to NATS KV store", c.configs.Server, c.configs.Bucket)
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	c.logger.Debugf("%s:%s Fetching value for key '%s'", c.configs.Server, c.configs.Bucket, key)

	span := c.addTrace(ctx, "get", key)
	defer c.sendOperationStats(time.Now(), "GET", "get", span, key)

	entry, err := c.kv.Get(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			c.logger.Debugf("%s:%s Key not found: '%s'", c.configs.Server, c.configs.Bucket, key)
			return "", fmt.Errorf("%w: %s", errKeyNotFound, key)
		}

		c.logger.Debugf("%s:%s Successfully retrieved value for key '%s'", c.configs.Server, c.configs.Bucket, key)

		return "", fmt.Errorf("failed to get key: %w", err)
	}

	return string(entry.Value()), nil
}

func (c *Client) Set(ctx context.Context, key, value string) error {
	c.logger.Debugf("%s:%s Setting value for key '%s'", c.configs.Server, c.configs.Bucket, key)

	span := c.addTrace(ctx, "set", key)
	defer c.sendOperationStats(time.Now(), "SET", "set", span, key, value)

	_, err := c.kv.Put(key, []byte(value))
	if err != nil {
		c.logger.Debugf("%s:%s Failed to set value for key '%s': %v", c.configs.Server, c.configs.Bucket, key, err)
		return fmt.Errorf("failed to set key-value pair: %w", err)
	}

	c.logger.Debugf("%s:%s Successfully set value for key '%s'", c.configs.Server, c.configs.Bucket, key)

	return nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	span := c.addTrace(ctx, "delete", key)
	defer c.sendOperationStats(time.Now(), "DELETE", "delete", span, key)

	err := c.kv.Delete(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			c.logger.Debugf("%s:%s Key not found for deletion: '%s'", c.configs.Server, c.configs.Bucket, key)
			return fmt.Errorf("%w: %s", errKeyNotFound, key)
		}

		c.logger.Debugf("%s:%s Failed to delete key '%s': %v", c.configs.Server, c.configs.Bucket, key, err)

		return fmt.Errorf("failed to delete key: %w", err)
	}

	c.logger.Debugf("%s:%s Successfully deleted key '%s'", c.configs.Server, c.configs.Bucket, key)

	return nil
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(context.Context) (any, error) {
	c.logger.Debugf("%s:%s Performing health check", c.configs.Server, c.configs.Bucket)

	h := &Health{
		Details: make(map[string]any),
	}

	h.Details["url"] = c.configs.Server
	h.Details["bucket"] = c.configs.Bucket

	_, err := c.js.AccountInfo()
	if err != nil {
		h.Status = "DOWN"

		c.logger.Debugf("%s:%s Health check failed: %v", c.configs.Server, c.configs.Bucket, err)

		return h, errStatusDown
	}

	h.Status = "UP"

	c.logger.Debugf("%s:%s Health check successful", c.configs.Server, c.configs.Bucket)

	return h, nil
}

func (c *Client) sendOperationStats(start time.Time, methodType, method string, span trace.Span, kv ...string) {
	duration := time.Since(start).Microseconds()

	c.logger.Debug(&Log{
		Type:     methodType,
		Duration: duration,
		Key:      strings.Join(kv, " "),
	})

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("natskv.%v.duration(μs)", method), duration))
	}

	c.metrics.RecordHistogram(context.Background(), "app_natskv_stats", float64(duration),
		"bucket", c.configs.Bucket,
		"type", methodType)
}

func (c *Client) addTrace(ctx context.Context, method, key string) trace.Span {
	if c.tracer != nil {
		_, span := c.tracer.Start(ctx, fmt.Sprintf("natskv-%v", method))
		span.SetAttributes(attribute.String("natskv.key", key))

		return span
	}

	return nil
}
