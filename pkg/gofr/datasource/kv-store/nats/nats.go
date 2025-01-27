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

func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

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

	js, err := nc.JetStream()
	if err != nil {
		c.logger.Errorf("error while initializing JetStream: %v", err)
		return
	}
	c.js = js

	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: c.configs.Bucket,
	})
	if err != nil {
		c.logger.Errorf("error while creating/accessing KV bucket: %v", err)
		return
	}
	c.kv = kv

	c.logger.Infof("connected to NATS at %v with bucket %v", c.configs.Server, c.configs.Bucket)
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	span := c.addTrace(ctx, "get", key)
	defer c.sendOperationStats(time.Now(), "GET", "get", span, key)

	entry, err := c.kv.Get(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return "", fmt.Errorf("key not found: %s", key)
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
			return fmt.Errorf("key not found: %s", key)
		}
		return fmt.Errorf("failed to delete key: %w", err)
	}

	return nil
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(context.Context) (any, error) {
	h := &Health{
		Details: make(map[string]any),
	}

	h.Details["url"] = c.configs.Server
	h.Details["bucket"] = c.configs.Bucket

	_, err := c.js.AccountInfo()
	if err != nil {
		h.Status = "DOWN"
		c.logger.Debugf("JetStream health check failed: %v", err)
		return h, errStatusDown
	}

	h.Status = "UP"
	return h, nil
}

func (c *Client) sendOperationStats(start time.Time, methodType string, method string, span trace.Span, kv ...string) {
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
