package cassandra

import (
	"time"
)

func (c *Client) BatchQuery(name, stmt string, values ...any) error {
	b, ok := c.cassandra.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	b.Query(stmt, values...)

	return nil
}

func (c *Client) ExecuteBatch(name string) error {
	defer c.logQueryAndSendMetrics(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now())

	b, ok := c.cassandra.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	return c.cassandra.session.executeBatch(b)
}

func (c *Client) ExecuteBatchCAS(name string, dest ...any) (bool, error) {
	defer c.logQueryAndSendMetrics(&QueryLog{Query: "batch", Keyspace: c.config.Keyspace}, time.Now())

	b, ok := c.cassandra.batches[name]
	if !ok {
		return false, errBatchNotInitialised
	}

	return c.cassandra.session.executeBatchCAS(b, dest...)
}
