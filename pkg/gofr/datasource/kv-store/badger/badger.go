package badger

import (
	"context"
	"errors"

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

// UseLogger sets the logger for the badgerDB client which asserts the Logger interface.
func (c *client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the badgerDB client which asserts the Metrics interface.
func (c *client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// Connect establishes a connection to badgerDB and registers metrics using the provided configuration when the client was Created.
func (c *client) Connect() {
	db, err := badger.Open(badger.DefaultOptions(c.configs.DirPath))
	if err != nil {
		c.logger.Errorf("failed to initialize badgerDB : %v", err)
	}

	c.db = db
}

func (c *client) Get(_ context.Context, key string) (string, error) {
	var value []byte

	//  transaction is set to false as we don't want to make any changes to data.
	txn := c.db.NewTransaction(false)
	defer txn.Discard()

	item, err := txn.Get([]byte(key))
	if err != nil {
		return "", err
	}

	value, err = item.ValueCopy(nil)
	if err != nil {
		return "", err
	}

	err = txn.Commit()
	if err != nil {
		return "", err
	}

	return string(value), nil
}

func (c *client) Set(_ context.Context, key string, value string) error {
	return c.useTransaction(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
}

func (c *client) Delete(_ context.Context, key string) error {
	return c.useTransaction(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

func (c *client) useTransaction(f func(txn *badger.Txn) error) error {
	txn := c.db.NewTransaction(true)
	defer txn.Discard()

	err := f(txn)
	if err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
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
