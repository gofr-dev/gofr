package badger

import (
	"context"
	"github.com/dgraph-io/badger/v4"
	"log"
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

func (c *client) HealthCheck(ctx context.Context) (any, error) {
	//TODO implement me
	panic("implement me")
}

func New(configs Configs) *client {
	return &client{configs: configs}
}

// UseLogger sets the logger for the MongoDB client which asserts the Logger interface.
func (c *client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the MongoDB client which asserts the Metrics interface.
func (c *client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// Connect establishes a connection to MongoDB and registers metrics using the provided configuration when the client was Created.
func (c *client) Connect() {
	db, err := badger.Open(badger.DefaultOptions(c.configs.DirPath))
	if err != nil {
		log.Fatal(err)
	}

	c.db = db
}

func (c *client) Get(_ context.Context, key string) (string, error) {
	var value []byte

	txn := c.db.NewTransaction(false)
	defer txn.Discard()

	item, err := txn.Get([]byte(key))
	if err != nil {
		return "", err
	}

	_, err = item.ValueCopy(value)
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
	txn := c.db.NewTransaction(true)
	defer txn.Discard()

	err := txn.Set([]byte(key), []byte(value))
	if err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *client) Delete(_ context.Context, key string) error {
	txn := c.db.NewTransaction(true)
	defer txn.Discard()

	err := txn.Delete([]byte(key))
	if err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}
