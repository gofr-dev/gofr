package badger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var errStatusDown = errors.New("status down")

type Configs struct {
	DirPath string
}

type Client struct {
	db      *badger.DB
	configs *Configs
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

func New(configs Configs) *Client {
	return &Client{configs: &configs}
}

// UseLogger sets the logger for the BadgerDB client which asserts the Logger interface.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the BadgerDB client which asserts the Metrics interface.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for BadgerDB client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// Connect establishes a connection to BadgerDB and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	c.logger.Debugf("connecting to BadgerDB at %v", c.configs.DirPath)

	badgerBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_badger_stats", "Response time of Badger queries in milliseconds.", badgerBuckets...)

	db, err := badger.Open(badger.DefaultOptions(c.configs.DirPath))
	if err != nil {
		c.logger.Errorf("error while connecting to BadgerDB: %v", err)
		return
	}

	c.db = db

	c.logger.Infof("connected to BadgerDB at %v", c.configs.DirPath)
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	span := c.addTrace(ctx, "get", key)

	defer c.sendOperationStats(time.Now(), "GET", "get", span, key)

	var value []byte

	// transaction is set to false as we don't want to make any changes to data.
	txn := c.db.NewTransaction(false)
	defer txn.Discard()

	item, err := txn.Get([]byte(key))
	if err != nil {
		c.logger.Debugf("error while fetching data for key: %v, error: %v", key, err)

		return "", err
	}

	value, err = item.ValueCopy(nil)
	if err != nil {
		c.logger.Debugf("error while reading value for key: %v, error: %v", key, err)

		return "", err
	}

	err = txn.Commit()
	if err != nil {
		c.logger.Debugf("error while committing transaction: %v", err)

		return "", err
	}

	return string(value), nil
}

func (c *Client) Set(ctx context.Context, key, value string) error {
	span := c.addTrace(ctx, "set", key)

	defer c.sendOperationStats(time.Now(), "SET", "set", span, key, value)

	return c.useTransaction(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
}

func (c *Client) Delete(ctx context.Context, key string) error {
	span := c.addTrace(ctx, "delete", key)

	defer c.sendOperationStats(time.Now(), "DELETE", "delete", span, key, "")

	return c.useTransaction(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

func (c *Client) useTransaction(f func(txn *badger.Txn) error) error {
	txn := c.db.NewTransaction(true)
	defer txn.Discard()

	err := f(txn)
	if err != nil {
		c.logger.Debugf("error while executing transaction: %v", err)

		return err
	}

	err = txn.Commit()
	if err != nil {
		c.logger.Debugf("error while committing transaction: %v", err)

		return err
	}

	return nil
}

func (c *Client) sendOperationStats(start time.Time, methodType string, method string,
	span trace.Span, kv ...string) {
	duration := time.Since(start).Microseconds()

	c.logger.Debug(&Log{
		Type:     methodType,
		Duration: duration,
		Key:      strings.Join(kv, " "),
	})

	if span != nil {
		defer span.End()

		span.SetAttributes(attribute.Int64(fmt.Sprintf("badger.%v.duration(μs)", method), time.Since(start).Microseconds()))
	}

	c.metrics.RecordHistogram(context.Background(), "app_badger_stats", float64(duration), "database", c.configs.DirPath,
		"type", methodType)
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(context.Context) (any, error) {
	h := Health{
		Details: make(map[string]any),
	}

	h.Details["location"] = c.configs.DirPath

	closed := c.db.IsClosed()
	if closed {
		h.Status = "DOWN"

		return &h, errStatusDown
	}

	h.Status = "UP"

	return &h, nil
}

func (c *Client) addTrace(ctx context.Context, method, key string) trace.Span {
	if c.tracer != nil {
		_, span := c.tracer.Start(ctx, fmt.Sprintf("badger-%v", method))

		span.SetAttributes(
			attribute.String("badger.key", key),
		)

		return span
	}

	return nil
}
