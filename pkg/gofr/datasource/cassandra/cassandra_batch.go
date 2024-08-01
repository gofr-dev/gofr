package cassandra

import (
	"time"
)

func (c *Client) BatchQuery(name, stmt string, values ...any) error {
	b, ok := c.cassandra.batches[name]
	if !ok {
		return ErrBatchNotInitialised
	}

	b.Query(stmt, values...)

	return nil
}

func (c *Client) ExecuteBatch(name string) error {
	defer c.postProcess(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now())

	b, ok := c.cassandra.batches[name]
	if !ok {
		return ErrBatchNotInitialised
	}

	return c.cassandra.session.executeBatch(b)
}

func (c *Client) ExecuteBatchCAS(name string, dest ...any) (bool, error) {
	defer c.postProcess(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now())

	b, ok := c.cassandra.batches[name]
	if !ok {
		return false, ErrBatchNotInitialised
	}

	return c.cassandra.session.executeBatchCAS(b, dest...)
}
