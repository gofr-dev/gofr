package datastore

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"gofr.dev/pkg/errors"
)

// Query executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
func (c *SQLClient) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if c == nil || c.DB == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()
	rows, err := c.DB.Query(query, args...)

	c.monitorQuery(begin, query)

	return rows, err
}

// Exec executes a query without returning any rows.
// The args are for any placeholder parameters in the query.
func (c *SQLClient) Exec(query string, args ...interface{}) (sql.Result, error) {
	if c == nil || c.DB == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()
	rows, err := c.DB.Exec(query, args...)

	c.monitorQuery(begin, query)

	return rows, err
}

// QueryRow executes a query that is expected to return at most one row.
// QueryRow always returns a non-nil value. Errors are deferred until
// Row's Scan method is called.
func (c *SQLClient) QueryRow(query string, args ...interface{}) *sql.Row {
	begin := time.Now()

	row := c.DB.QueryRow(query, args...)

	c.monitorQuery(begin, query)

	return row
}

// QueryContext executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
func (c *SQLClient) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if c == nil || c.DB == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()

	rows, err := c.DB.QueryContext(ctx, query, args...)

	c.monitorQuery(begin, query)

	return rows, err
}

// ExecContext executes a query without returning any rows.
// The args are for any placeholder parameters in the query.
func (c *SQLClient) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if c == nil || c.DB == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()

	rows, err := c.DB.ExecContext(ctx, query, args...)

	c.monitorQuery(begin, query)

	return rows, err
}

// QueryRowContext executes a query that is expected to return at most one row.
// QueryRowContext always returns a non-nil value. Errors are deferred until
// Row's Scan method is called.
func (c *SQLClient) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	begin := time.Now()
	row := c.DB.QueryRowContext(ctx, query, args...)

	c.monitorQuery(begin, query)

	return row
}

func checkQueryOperation(query string) string {
	query = strings.ToLower(query)
	query = strings.TrimSpace(query)

	if strings.HasPrefix(query, "select") {
		return "SELECT"
	} else if strings.HasPrefix(query, "update") {
		return "UPDATE"
	} else if strings.HasPrefix(query, "delete") {
		return "DELETE"
	} else if strings.HasPrefix(query, "commit") {
		return "COMMIT"
	} else if strings.HasPrefix(query, "begin") {
		return "BEGIN"
	} else if strings.HasPrefix(query, "set") {
		return "SET"
	}

	return "INSERT"
}

func (c *SQLClient) monitorQuery(begin time.Time, query string) {
	if c == nil || c.DB == nil {
		return
	}

	var (
		hostName string
		dbName   string
	)

	dur := time.Since(begin).Seconds()

	if c.config != nil {
		hostName = c.config.HostName
		dbName = c.config.Database
	}

	// push stats to prometheus
	sqlStats.WithLabelValues(checkQueryOperation(query), hostName, dbName).Observe(dur)

	ql := QueryLogger{
		Query:     []string{query},
		DataStore: SqlStore,
	}

	// log the query
	if c.logger != nil {
		ql.Duration = time.Since(begin).Microseconds()
		c.logger.Debug(ql)
	}
}

// Begin starts a transaction. The default isolation level is dependent on
// the driver.
func (c *SQLClient) Begin() (*SQLTx, error) {
	if c == nil || c.DB == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()

	tx, err := c.DB.Begin()
	c.monitorQuery(begin, "BEGIN")

	return &SQLTx{Tx: tx, logger: c.logger, config: c.config}, err
}

// BeginTx starts a transaction.
func (c *SQLClient) BeginTx(ctx context.Context, opts *sql.TxOptions) (*SQLTx, error) {
	if c == nil || c.DB == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()

	tx, err := c.DB.BeginTx(ctx, opts)
	c.monitorQuery(begin, "BEGIN TRANSACTION")

	return &SQLTx{Tx: tx, logger: c.logger, config: c.config}, err
}

// Exec executes a query that doesn't return rows.
func (c *SQLTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	if c == nil || c.Tx == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()

	result, err := c.Tx.Exec(query, args...)
	c.monitorQuery(begin, query)

	return result, err
}

// Query executes a query that returns rows, typically a SELECT.
func (c *SQLTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if c == nil || c.Tx == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()

	rows, err := c.Tx.Query(query, args...)
	c.monitorQuery(begin, query)

	return rows, err
}

// QueryRow executes a query that is expected to return at most one row.
// QueryRow always returns a non-nil value. Errors are deferred until
// Row's Scan method is called.
func (c *SQLTx) QueryRow(query string, args ...interface{}) *sql.Row {
	begin := time.Now()

	row := c.Tx.QueryRow(query, args...)
	c.monitorQuery(begin, query)

	return row
}

// ExecContext executes a query that doesn't return rows.
func (c *SQLTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if c == nil || c.Tx == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()

	result, err := c.Tx.ExecContext(ctx, query, args...)
	c.monitorQuery(begin, query)

	return result, err
}

// QueryContext executes a query that returns rows, typically a SELECT.
func (c *SQLTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if c == nil || c.Tx == nil {
		return nil, errors.SQLNotInitialized
	}

	begin := time.Now()

	rows, err := c.Tx.QueryContext(ctx, query, args...)
	c.monitorQuery(begin, query)

	return rows, err
}

// QueryRowContext executes a query that is expected to return at most one row.
// QueryRowContext always returns a non-nil value. Errors are deferred until
// Row's Scan method is called.
func (c *SQLTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	begin := time.Now()

	row := c.Tx.QueryRowContext(ctx, query, args...)
	c.monitorQuery(begin, query)

	return row
}

// Commit commits the transaction.
func (c *SQLTx) Commit() error {
	if c == nil || c.Tx == nil {
		return errors.SQLNotInitialized
	}

	begin := time.Now()

	err := c.Tx.Commit()
	c.monitorQuery(begin, "COMMIT")

	return err
}

func (c *SQLTx) monitorQuery(begin time.Time, query string) {
	var (
		hostName string
		dbName   string
	)

	dur := time.Since(begin).Seconds()

	if c.config != nil {
		hostName = c.config.HostName
		dbName = c.config.Database
	}

	var ql QueryLogger

	ql.Query = append(ql.Query, query)
	ql.Duration = time.Since(begin).Microseconds()
	ql.StartTime = begin
	ql.DataStore = SqlStore

	// push stats to prometheus
	sqlStats.WithLabelValues(checkQueryOperation(query), hostName, dbName).Observe(dur)

	// log the query
	if c.logger != nil {
		c.logger.Debug(ql)
	}
}
