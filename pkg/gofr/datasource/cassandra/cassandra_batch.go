package cassandra

import (
	"time"
)

func (c *Client) BatchQuery(name, stmt string, values ...any) error {
	span := c.addTrace("batch-query", stmt)

	defer c.sendOperationStats(&QueryLog{Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "batch-query", span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	b.Query(stmt, values...)

	return nil
}

func (c *Client) ExecuteBatch(name string) error {
	span := c.addTrace("execute-batch", "batch")

	defer c.sendOperationStats(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now(), "execute-batch",
		span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	return c.cassandra.session.executeBatch(b)
}

func (c *Client) ExecuteBatchCAS(name string, dest ...any) (bool, error) {
	span := c.addTrace("execute-batch-cas", "batch")

	defer c.sendOperationStats(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now(), "execute-batch-cas",
		span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return false, errBatchNotInitialised
	}

	return c.cassandra.session.executeBatchCAS(b, dest...)
}
