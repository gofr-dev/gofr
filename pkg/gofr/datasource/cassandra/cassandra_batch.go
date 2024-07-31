package cassandra

import (
	"time"
)

func (c *Client) BatchQuery(name, stmt string, values ...any) error {
	if b, ok := c.cassandra.batches[name]; ok {
		b.Query(stmt, values...)

		return nil
	}

	return ErrBatchNotInitialised
}

func (c *Client) ExecuteBatch(name string) error {
	defer c.postProcess(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now())

	if b, ok := c.cassandra.batches[name]; ok {
		return c.cassandra.session.executeBatch(b)
	}

	return ErrBatchNotInitialised
}

//nolint:exhaustive // We just want to take care of slice and struct in this case.
func (c *Client) ExecuteBatchCAS(name string, dest ...any) (bool, error) {
	defer c.postProcess(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now())

	if b, ok := c.cassandra.batches[name]; ok {
		return c.cassandra.session.executeBatchCAS(b, dest...)
	}

	return false, ErrBatchNotInitialised
}
