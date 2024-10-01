package cassandra

import (
	"context"
	"time"
)

func (c *Client) BatchQuery(ctx context.Context, name, stmt string, values ...any) error {
	_, span := c.addTrace(ctx, "batch-query", stmt)

	defer c.sendOperationStats(&QueryLog{Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "batch-query", span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	b.Query(stmt, values...)

	return nil
}

func (c *Client) ExecuteBatch(ctx context.Context, name string) error {
	_, span := c.addTrace(ctx, "execute-batch", "batch")

	defer c.sendOperationStats(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now(), "execute-batch",
		span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	return c.cassandra.session.executeBatch(b)
}

func (c *Client) ExecuteBatchCAS(ctx context.Context, name string, dest ...any) (bool, error) {
	_, span := c.addTrace(ctx, "execute-batch-cas", "batch")

	defer c.sendOperationStats(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now(), "execute-batch-cas",
		span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return false, errBatchNotInitialised
	}

	return c.cassandra.session.executeBatchCAS(b, dest...)
}
