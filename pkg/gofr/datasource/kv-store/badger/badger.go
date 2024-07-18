package badger

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type Configs struct {
	DirPath string
}

type client struct {
	db      *badger.DB
	configs Configs
	logger  Logger
	metrics Metrics
}

func New(configs Configs) *client {
	return &client{configs: configs}
}

// UseLogger sets the logger for the BadgerDB client which asserts the Logger interface.
func (c *client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the BadgerDB client which asserts the Metrics interface.
func (c *client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// Connect establishes a connection to BadgerDB and registers metrics using the provided configuration when the client was Created.
func (c *client) Connect() {
	c.logger.Infof("connecting to BadgerDB at %v", c.configs.DirPath)

	badgerBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_badger_stats", "Response time of Badger queries in milliseconds.", badgerBuckets...)

	db, err := badger.Open(badger.DefaultOptions(c.configs.DirPath))
	if err != nil {
		c.logger.Errorf("error while connecting to BadgerDB: %v", err)
	}

	c.db = db
}

func (c *client) Get(_ context.Context, key string) (string, error) {
	defer c.logQueryAndSendMetrics(time.Now(), "GET", key, "")

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
		c.logger.Debugf("error while commiting transaction: %v", err)

		return "", err
	}

	return string(value), nil
}

func (c *client) Set(_ context.Context, key, value string) error {
	defer c.logQueryAndSendMetrics(time.Now(), "SET", key, value)

	return c.useTransaction(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
}

func (c *client) Delete(_ context.Context, key string) error {
	defer c.logQueryAndSendMetrics(time.Now(), "DELETE", key, "")

	return c.useTransaction(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

func (c *client) useTransaction(f func(txn *badger.Txn) error) error {
	txn := c.db.NewTransaction(true)
	defer txn.Discard()

	err := f(txn)
	if err != nil {
		c.logger.Debugf("error while executing transaction: %v", err)

		return err
	}

	err = txn.Commit()
	if err != nil {
		c.logger.Debugf("error while commiting transaction: %v", err)

		return err
	}

	return nil
}

func (c *client) logQueryAndSendMetrics(start time.Time, methodType string, kv ...string) {
	duration := time.Since(start).Milliseconds()

	c.logger.Debug(&Log{
		Type:     methodType,
		Duration: duration,
		Key:      strings.Join(kv, " "),
	})

	c.metrics.RecordHistogram(context.Background(), "app_badger_stats", float64(duration), "database", c.configs.DirPath,
		"type", methodType)
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (c *client) HealthCheck(context.Context) (any, error) {
	h := Health{
		Details: make(map[string]any),
	}

	h.Details["location"] = c.configs.DirPath

	closed := c.db.IsClosed()
	if closed {
		h.Status = "DOWN"

		return &h, errors.New("status down")
	}

	h.Status = "UP"

	return &h, nil
}
