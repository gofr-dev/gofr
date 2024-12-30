package scylladb

import (
	"context"
	"errors"
	"fmt"
	"github.com/gocql/gocql"
	_ "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"reflect"

	"go.opentelemetry.io/otel/trace"

	"time"
)

const (
	LoggedBatch = iota
	UnloggedBatch
	CounterBatch
)

var errStatusDown = errors.New("status down")

type Config struct {
	Hosts    string
	Keyspace string
	Port     int
	Username string
	Password string
}

type scylladb struct {
	clusterConfig clusterConfig
	session       session
	query         query
	batches       map[string]batch
}
type Client struct {
	config *Config

	scylla *scylladb

	session session

	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

type Health struct {
	Status  string         `json:" status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func New(conf Config) *Client {
	cass := &scylladb{clusterConfig: newClusterConfig(&conf)}

	return &Client{config: &conf, scylla: cass}
}

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
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

func (c *Client) Query(dest any, stmt string, values ...any) error {
	return c.QueryWithCtx(context.Background(), dest, stmt, values...)

}

func (c *Client) ExecCAS(dest any, stmt string, values ...any) (bool, error) {
	return c.ExecCASWithCtx(context.Background(), dest, stmt, values)
}
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
	q := c.scylla.session.query(stmt, values...)

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
		applied, err = q.scanCAS(rv.Interface())
	}

	return applied, err
}
func (c *Client) rowsToStructCAS(query query, vo reflect.Value) (bool, error) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	row := make(map[string]any)

	applied, err := query.mapScanCAS(row)
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

func (c *Client) QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error {
	span := c.addTrace(ctx, "query", stmt)

	defer c.sendOperationStats(&QueryLog{Operation: "QueryWithCtx", Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "query", span)

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		c.logger.Error("we did not get a pointer. data is not settable.")

		return errDestinationIsNotPointer
	}

	rv := rvo.Elem()
	iter := c.scylla.session.query(stmt, values...).iter()

	switch rv.Kind() {
	case reflect.Slice:
		numRows := iter.numRows()

		for numRows > 0 {
			val := reflect.New(rv.Type().Elem())

			if rv.Type().Elem().Kind() == reflect.Struct {
				c.rowsToStruct(iter, val)
			} else {
				_ = iter.scan(val.Interface())
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

func (c *Client) Exec(stmt string, values ...any) error {
	return c.ExecWithCtx(context.Background(), stmt, values)
}

func (c *Client) ExecWithCtx(ctx context.Context, stmt string, values ...any) error {
	span := c.addTrace(ctx, "exec", stmt)
	defer c.sendOperationStats(&QueryLog{Operation: "ExecWithCtx", Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "exec", span)

	return c.session.query(stmt, values...).exec()
}

func (c *Client) NewBatch(name string, batchType int) error {
	return c.NewBatchWithCtx(context.Background(), name, batchType)
}
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

func (c *Client) BatchQuery(name, stmt string, values ...any) error {
	return c.BatchQueryWithCtx(context.Background(), name, stmt, values...)
}

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

func (c *Client) rowsToStruct(iter iterator, vo reflect.Value) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	columns := c.getColumnsFromColumnsInfo(iter.columns())
	fieldNameIndex := c.getFieldNameIndex(v)
	fields := c.getFields(columns, fieldNameIndex, v)

	_ = iter.scan(fields...)

	if vo.CanSet() {
		vo.Set(v)
	}
}
func (*Client) getColumnsFromColumnsInfo(columns []gocql.ColumnInfo) []string {
	cols := make([]string, 0)

	for _, column := range columns {
		cols = append(cols, column.Name)
	}

	return cols
}

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

	err := c.scylla.session.query("SELECT now() FROM system.local").exec()
	if err != nil {
		h.Status = statusDown
		h.Details["message"] = err.Error()

		return &h, errStatusDown
	}

	h.Status = statusUp

	return &h, nil
}

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

func (c *Client) getFields(columns []string, fieldNameIndex map[string]int, v reflect.Value) []interface{} {
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

func (c *Client) ExecuteBatchCAS(name string, dest ...any) (bool, error) {
	return c.ExecuteBatchCASWithCtx(context.Background(), name, dest)
}
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
