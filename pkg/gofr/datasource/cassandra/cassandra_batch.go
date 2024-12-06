package cassandra

import (
	"context"
	"time"
)

func (c *Client) BatchQuery(name, stmt string, values ...any) error {
	return c.BatchQueryWithCtx(context.Background(), name, stmt, values...)
}

func (c *Client) ExecuteBatch(name string) error {
	return c.ExecuteBatchWithCtx(context.Background(), name)
}

func (c *Client) ExecuteBatchCAS(name string, dest ...any) (bool, error) {
	return c.ExecuteBatchCASWithCtx(context.Background(), name, dest)
}

func (c *Client) BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error {
	span := c.addTrace(ctx, "batch-query", stmt)

	defer c.sendOperationStats(&QueryLog{
		Operation: "BatchQueryWithCtx",
		Query:     stmt,
		Keyspace:  c.config.Keyspace,
	}, time.Now(), "batch-query", span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	b.Query(stmt, values...)

	return nil
}

func (c *Client) ExecuteBatchWithCtx(ctx context.Context, name string) error {
	span := c.addTrace(ctx, "execute-batch", "batch")

	defer c.sendOperationStats(&QueryLog{
		Operation: "ExecuteBatchWithCtx",
		Query:     "batch",
		Keyspace:  c.config.Keyspace,
	}, time.Now(), "execute-batch", span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	return c.cassandra.session.executeBatch(b)
}

func (c *Client) ExecuteBatchCASWithCtx(ctx context.Context, name string, dest ...any) (bool, error) {
	span := c.addTrace(ctx, "execute-batch-cas", "batch")

	defer c.sendOperationStats(&QueryLog{
		Operation: "ExecuteBatchCASWithCtx",
		Query:     "batch",
		Keyspace:  c.config.Keyspace,
	}, time.Now(), "execute-batch-cas", span)

	b, ok := c.cassandra.batches[name]
	if !ok {
		return false, errBatchNotInitialised
	}

	return c.cassandra.session.executeBatchCAS(b, dest...)
}
