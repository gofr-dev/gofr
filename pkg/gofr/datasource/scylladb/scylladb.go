package scylladb

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/gocql/gocql"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	LoggedBatch = iota
	UnloggedBatch
	CounterBatch
)

var errStatusDown = errors.New("status down")

// Config holds the configuration settings for connecting to a ScyllaDB cluster,
// including host addresses, keyspace, port, and authentication credentials.
type Config struct {
	Hosts    string
	Keyspace string
	Port     int
	Username string
	Password string
}

// Scylladb represents the connection and operations context for interacting with a ScyllaDB cluster,
// including configuration, active session, query handling, and initialized batches.
type Scylladb struct {
	clusterConfig clusterConfig
	session       session
	query         query
	batches       map[string]batch
}

// Client is the main interface for interacting with a ScyllaDB cluster,
// managing configuration, ScyllaDB operations, logging, metrics, and tracing.
type Client struct {
	config *Config

	scylla *Scylladb
	logger Logger

	metrics Metrics

	tracer trace.Tracer
}

// Health represents the health status of the ScyllaDB cluster,
// including the overall status (e.g., "UP" or "DOWN") and additional details.
type Health struct {
	Status  string         `json:" status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// New initializes ScyllaDB driver with the provided configuration.
func New(conf Config) *Client {
	cass := &Scylladb{clusterConfig: newClusterConfig(&conf)}

	return &Client{config: &conf, scylla: cass}
}

// Connect establishes a connection to Scylladb.
func (c *Client) Connect() {
	c.logger.Debugf("Connecting to ScyllaDB at %v on port %v to keyspace %v", c.config.Hosts, c.config.Port, c.config.Keyspace)
	sess, err := c.scylla.clusterConfig.createSession()

	if err != nil {
		c.logger.Error("failed to connect to ScyllaDB:", err)
		return
	}

	scyllaBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_scylla_stats", "Response time of scylla queries in microseconds", scyllaBuckets...)

	c.logger.Logf("connected to '%s' keyspace at host '%s' and port '%d'", c.config.Keyspace, c.config.Hosts, c.config.Port)
	c.scylla.session = sess
}

// UseLogger sets the logger for the scylladb client.
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the scylladb client.
func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the scylladb client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// Query is the original method without context.
// It internally delegates to QueryWithCtx using context.Background() as the default context.
func (c *Client) Query(dest any, stmt string, values ...any) error {
	return c.QueryWithCtx(context.Background(), dest, stmt, values...)
}

// Exec executes a CQL (Cassandra Query Language) statement on a ScyllaDB cluster
// with the provided values, using the default context (context.Background()).
func (c *Client) Exec(stmt string, values ...any) error {
	return c.ExecWithCtx(context.Background(), stmt, values)
}

// ExecWithCtx executes a CQL statement by using the context,statement,values and returns error.
func (c *Client) ExecWithCtx(ctx context.Context, stmt string, values ...any) error {
	span := c.addTrace(ctx, "exec", stmt)
	defer c.sendOperationStats(&QueryLog{Operation: "ExecWithCtx", Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "exec", span)

	return c.scylla.session.Query(stmt, values...).Exec()
}

// ExecCAS performs Compare and Set operation on ScyllaDB cluster.
func (c *Client) ExecCAS(dest any, stmt string, values ...any) (bool, error) {
	return c.ExecCASWithCtx(context.Background(), dest, stmt, values)
}

// ExecCASWithCtx takes default context,destination,statement,values and  return bool and error.
//
//nolint:exhaustive // We just want to take care of slice and struct in this case.
func (c *Client) ExecCASWithCtx(ctx context.Context, dest any, stmt string, values ...any) (bool, error) {
	var (
		applied bool
		err     error
	)

	span := c.addTrace(ctx, "exec-cas", stmt)

	defer c.sendOperationStats(&QueryLog{Operation: "ExecCASWithCtx", Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "exec-cas", span)

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		c.logger.Debugf("we did not get a pointer. data is not settable.")

		return false, errDestinationIsNotPointer
	}

	rv := rvo.Elem()
	q := c.scylla.session.Query(stmt, values...)

	switch rv.Kind() {
	case reflect.Struct:
		applied, err = c.rowsToStructCAS(q, rv)

	case reflect.Slice:
		c.logger.Debugf("a slice of %v was not expected.", reflect.SliceOf(reflect.TypeOf(dest)).String())

		return false, errUnexpectedSlice{target: reflect.SliceOf(reflect.TypeOf(dest)).String()}

	case reflect.Map:
		c.logger.Debugf("a map was not expected.")

		return false, errUnexpectedMap

	default:
		applied = true
	}

	return applied, err
}

// rowsToStructCAS Scans a CAS query result into a struct, setting fields based on column names and types,
// and returns if the update was applied.
func (c *Client) rowsToStructCAS(query query, vo reflect.Value) (bool, error) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	row := make(map[string]any)

	applied, err := query.MapScanCAS(row)
	if err != nil {
		return false, err
	}

	fieldNameIndex := c.getFieldNameIndex(v)

	for col, value := range row {
		if i, ok := fieldNameIndex[col]; ok {
			field := v.Field(i)
			if reflect.TypeOf(value) == field.Type() {
				field.Set(reflect.ValueOf(value))
			}
		}
	}

	if vo.CanSet() {
		vo.Set(v)
	}

	return applied, nil
}

// QueryWithCtx takes context ,destination,statement,values and returns error.
//
//nolint:exhaustive // We just want to take care of slice and struct in this case
func (c *Client) QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error {
	span := c.addTrace(ctx, "query", stmt)

	defer c.sendOperationStats(&QueryLog{Operation: "QueryWithCtx", Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "query", span)

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		c.logger.Error("we did not get a pointer. data is not settable.")
		return errDestinationIsNotPointer
	}

	rv := rvo.Elem()
	iter := c.scylla.session.Query(stmt, values...).Iter()

	switch rv.Kind() {
	case reflect.Slice:
		numRows := iter.NumRows()

		for numRows > 0 {
			val := reflect.New(rv.Type().Elem())

			if rv.Type().Elem().Kind() == reflect.Struct {
				c.rowsToStruct(iter, val)
			} else {
				_ = iter.Scan(val.Interface())
			}

			rv = reflect.Append(rv, val.Elem())

			numRows--
		}

		if rvo.Elem().CanSet() {
			rvo.Elem().Set(rv)
		}

	case reflect.Struct:
		c.rowsToStruct(iter, rv)

	default:
		c.logger.Debugf("a pointer to %v was not expected.", rv.Kind().String())

		return errUnexpectedPointer{target: rv.Kind().String()}
	}

	return nil
}

// NewBatch creates a new batch operation for a ScyllaDB cluster with the provided
// name and batch type, using the default context (context.Background()).
func (c *Client) NewBatch(name string, batchType int) error {
	return c.NewBatchWithCtx(context.Background(), name, batchType)
}

// NewBatchWithCtx uses context ,name ,batchType and returns error
func (c *Client) NewBatchWithCtx(_ context.Context, name string, batchType int) error {
	switch batchType {
	case LoggedBatch, UnloggedBatch, CounterBatch:
		if len(c.scylla.batches) == 0 {
			c.scylla.batches = make(map[string]batch)
		}

		c.scylla.batches[name] = c.scylla.session.newBatch(gocql.BatchType(batchType))

		return nil
	default:
		return errUnsupportedBatchType
	}
}

// BatchQuery executes a batched query in a ScyllaDB cluster with the provided
// name, statement, and values, using the default context (context.Background()).
func (c *Client) BatchQuery(name, stmt string, values ...any) error {
	return c.BatchQueryWithCtx(context.Background(), name, stmt, values...)
}

// BatchQueryWithCtx executes Query with  the provided context,name,statement and values.
func (c *Client) BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error {
	span := c.addTrace(ctx, "batch-query", stmt)

	defer c.sendOperationStats(&QueryLog{
		Operation: "BatchQueryWithCtx",
		Query:     stmt,
		Keyspace:  c.config.Keyspace,
	}, time.Now(), "batch-query", span)

	b, ok := c.scylla.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	b.Query(stmt, values...)

	return nil
}

// ExecuteBatchWithCtx executes batch with provided context,name and returns err.
func (c *Client) ExecuteBatchWithCtx(ctx context.Context, name string) error {
	span := c.addTrace(ctx, "execute-batch", "batch")

	defer c.sendOperationStats(&QueryLog{
		Operation: "ExecuteBatchWithCtx",
		Query:     "batch",
		Keyspace:  c.config.Keyspace,
	}, time.Now(), "execute-batch", span)

	b, ok := c.scylla.batches[name]
	if !ok {
		return errBatchNotInitialised
	}

	return c.scylla.session.executeBatch(b)
}

// ExecuteBatchCAS executes a Compare and set operation on ScyllaDB cluster using the provided batch name.
func (c *Client) ExecuteBatchCAS(name string, dest ...any) (bool, error) {
	return c.ExecuteBatchCASWithCtx(context.Background(), name, dest)
}

// ExecuteBatchCASWithCtx takes default context,name,destination returns bool and error.
func (c *Client) ExecuteBatchCASWithCtx(ctx context.Context, name string, dest ...any) (bool, error) {
	span := c.addTrace(ctx, "execute-batch-cas", "batch")

	defer c.sendOperationStats(&QueryLog{
		Operation: "ExecuteBatchCASWithCtx",
		Query:     "batch",
		Keyspace:  c.config.Keyspace,
	}, time.Now(), "execute-batch-cas", span)

	b, ok := c.scylla.batches[name]
	if !ok {
		return false, errBatchNotInitialised
	}

	return c.scylla.session.executeBatchCAS(b, dest...)
}

// ExecuteBatch executes a previously initialized batch operation in a ScyllaDB cluster
// using the provided batch name, with the default context (context.Background()).
func (c *Client) ExecuteBatch(name string) error {
	return c.ExecuteBatchWithCtx(context.Background(), name)
}

// rowsToStruct Scans the iterator row data and maps it to the fields of the provided struct.
func (c *Client) rowsToStruct(iter iterator, vo reflect.Value) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	columns := c.getColumnsFromColumnsInfo(iter.Columns())
	fieldNameIndex := c.getFieldNameIndex(v)
	fields := getFields(columns, fieldNameIndex, v)

	_ = iter.Scan(fields...)

	if vo.CanSet() {
		vo.Set(v)
	}
}

// getColumnsFromColumnsInfo Extracts and returns a slice of column names from the provided gocql.ColumnInfo slice.
func (*Client) getColumnsFromColumnsInfo(columns []gocql.ColumnInfo) []string {
	cols := make([]string, 0)

	for _, column := range columns {
		cols = append(cols, column.Name)
	}

	return cols
}

// getFieldNameIndex Returns a map of field names from struct convert  toSnakeCase to their index positions in the struct.
func (*Client) getFieldNameIndex(v reflect.Value) map[string]int {
	fieldNameIndex := map[string]int{}

	for i := 0; i < v.Type().NumField(); i++ {
		var name string

		f := v.Type().Field(i)
		tag := f.Tag.Get("db")

		if tag != "" {
			name = tag
		} else {
			name = toSnakeCase(f.Name)
		}

		fieldNameIndex[name] = i
	}

	return fieldNameIndex
}

// HealthCheck performs a health check on the ScyllaDB cluster by querying.
func (c *Client) HealthCheck(context.Context) (any, error) {
	const (
		statusDown = "DOWN"
		statusUp   = "UP"
	)

	h := Health{
		Details: make(map[string]interface{}),
	}

	h.Details["host"] = c.config.Hosts
	h.Details["keyspace"] = c.config.Keyspace

	if c.scylla.session == nil {
		h.Status = statusDown
		h.Details["message"] = "cassandra not connected"

		return &h, errStatusDown
	}

	err := c.scylla.session.Query("SELECT now() FROM system.local").Exec()
	if err != nil {
		h.Status = statusDown
		h.Details["message"] = err.Error()

		return &h, errStatusDown
	}

	h.Status = statusUp

	return &h, nil
}

// addTrace starts a new trace span for the specified method and query.
func (c *Client) addTrace(ctx context.Context, method, query string) trace.Span {
	if c.tracer != nil {
		_, span := c.tracer.Start(ctx, fmt.Sprintf("scylladb-%v", method))

		span.SetAttributes(
			attribute.String("scylladb.query", query),
			attribute.String("scylladb.keyspace", c.config.Keyspace),
		)

		return span
	}

	return nil
}

// sendOperationStats Logs query duration and stats, records metrics, and ends the trace span if present.
func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("scylla.%v.duration", method), duration))
	}

	c.metrics.RecordHistogram(context.Background(), "app_scylla_stats", float64(duration), "hostname", c.config.Hosts,
		"keyspace", c.config.Keyspace)

	c.scylla.query = nil
}

// getFields returns a slice of field pointers from the struct, mapping columns to their corresponding fields.
func getFields(columns []string, fieldNameIndex map[string]int, v reflect.Value) []interface{} {
	fields := make([]interface{}, len(columns))

	for i, column := range columns {
		if index, ok := fieldNameIndex[column]; ok {
			fields[i] = v.Field(index).Addr().Interface()
		} else {
			fields[i] = new(interface{})
		}
	}

	return fields
}
