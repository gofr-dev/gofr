package cassandra

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

type Config struct {
	Hosts    string
	Keyspace string
	Port     int
	Username string
	Password string
}

type cassandra struct {
	clusterConfig clusterConfig
	session       session
	query         query
	batches       map[string]batch
}

type Client struct {
	config *Config

	cassandra *cassandra

	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

var errStatusDown = errors.New("status down")

// New initializes Cassandra driver with the provided configuration.
// The Connect method must be called to establish a connection to Cassandra.
// Usage:
//
//	client := New(config)
//	client.UseLogger(loggerInstance)
//	client.UseMetrics(metricsInstance)
//	client.Connect()
func New(conf Config) *Client {
	cass := &cassandra{clusterConfig: newClusterConfig(&conf)}

	return &Client{config: &conf, cassandra: cass}
}

// Connect establishes a connection to Cassandra and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	c.logger.Debugf("connecting to Cassandra at %v on port %v to keyspace %v", c.config.Hosts, c.config.Port, c.config.Keyspace)

	sess, err := c.cassandra.clusterConfig.createSession()
	if err != nil {
		c.logger.Error("error connecting to Cassandra: ", err)

		return
	}

	cassandraBucktes := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_cassandra_stats", "Response time of CASSANDRA queries in microseconds.", cassandraBucktes...)

	c.logger.Logf("connected to '%s' keyspace at host '%s' and port '%d'", c.config.Keyspace, c.config.Hosts, c.config.Port)

	c.cassandra.session = sess
}

// UseLogger sets the logger for the Cassandra client which asserts the Logger interface.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Cassandra client which asserts the Metrics interface.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for Cassandra client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// Query is the original method without context, preserved for backward compatibility.
// It internally delegates to [Client.QueryWithCtx] using context.Background() as the default context.
func (c *Client) Query(dest any, stmt string, values ...any) error {
	return c.QueryWithCtx(context.Background(), dest, stmt, values...)
}

func (c *Client) Exec(stmt string, values ...any) error {
	return c.ExecWithCtx(context.Background(), stmt, values)
}

func (c *Client) ExecCAS(dest any, stmt string, values ...any) (bool, error) {
	return c.ExecCASWithCtx(context.Background(), dest, stmt, values)
}

func (c *Client) NewBatch(name string, batchType int) error {
	return c.NewBatchWithCtx(context.Background(), name, batchType)
}

// QueryWithCtx executes a CQL query in the Cassandra database and returns the result.
//
//nolint:exhaustive // We just want to take care of slice and struct in this case.
func (c *Client) QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error {
	span := c.addTrace(ctx, "query", stmt)

	defer c.sendOperationStats(&QueryLog{Operation: "QueryWithCtx", Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "query", span)

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		c.logger.Error("we did not get a pointer. data is not settable.")

		return errDestinationIsNotPointer
	}

	rv := rvo.Elem()
	iter := c.cassandra.session.query(stmt, values...).iter()

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

func (c *Client) ExecWithCtx(ctx context.Context, stmt string, values ...any) error {
	span := c.addTrace(ctx, "exec", stmt)

	defer c.sendOperationStats(&QueryLog{Operation: "ExecWithCtx", Query: stmt, Keyspace: c.config.Keyspace}, time.Now(), "exec", span)

	return c.cassandra.session.query(stmt, values...).exec()
}

// ExecCASWithCtx executes a CQL query in the Cassandra database and returns the true if the query is applied.
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
	q := c.cassandra.session.query(stmt, values...)

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

func (c *Client) NewBatchWithCtx(_ context.Context, name string, batchType int) error {
	switch batchType {
	case LoggedBatch, UnloggedBatch, CounterBatch:
		if len(c.cassandra.batches) == 0 {
			c.cassandra.batches = make(map[string]batch)
		}

		c.cassandra.batches[name] = c.cassandra.session.newBatch(gocql.BatchType(batchType))

		return nil
	default:
		return errUnsupportedBatchType
	}
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

func (*Client) getFields(columns []string, fieldNameIndex map[string]int, v reflect.Value) []any {
	fields := make([]any, 0)

	for _, column := range columns {
		if i, ok := fieldNameIndex[column]; ok {
			fields = append(fields, v.Field(i).Addr().Interface())
		} else {
			var i any

			fields = append(fields, &i)
		}
	}

	return fields
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

func (*Client) getColumnsFromColumnsInfo(columns []gocql.ColumnInfo) []string {
	cols := make([]string, 0)

	for _, column := range columns {
		cols = append(cols, column.Name)
	}

	return cols
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	if span != nil {
		defer span.End()

		span.SetAttributes(attribute.Int64(fmt.Sprintf("cassandra.%v.duration", method), duration))
	}

	c.metrics.RecordHistogram(context.Background(), "app_cassandra_stats", float64(duration), "hostname", c.config.Hosts,
		"keyspace", c.config.Keyspace)

	c.cassandra.query = nil
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// HealthCheck checks the health of the Cassandra.
func (c *Client) HealthCheck(context.Context) (any, error) {
	const (
		statusDown = "DOWN"
		statusUp   = "UP"
	)

	h := Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = c.config.Hosts
	h.Details["keyspace"] = c.config.Keyspace

	if c.cassandra.session == nil {
		h.Status = statusDown
		h.Details["message"] = "cassandra not connected"

		return &h, errStatusDown
	}

	err := c.cassandra.session.query("SELECT now() FROM system.local").exec()
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
		_, span := c.tracer.Start(ctx, fmt.Sprintf("cassandra-%v", method))

		span.SetAttributes(
			attribute.String("cassandra.query", query),
			attribute.String("cassandra.keyspace", c.config.Keyspace),
		)

		return span
	}

	return nil
}
