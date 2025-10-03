package scylladb

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/stoewer/go-strcase"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// scylladbIterator implements iterator interface.
type scylladbIterator struct {
	iter *gocql.Iter
}

// Columns gets the Column information. This method wraps the `Columns` method of the underlying `iter` object.
func (s *scylladbIterator) Columns() []gocql.ColumnInfo {
	return s.iter.Columns()
}

// Scan gets the next row from the ScyllaDB iterator and fills in the provided arguments.
func (s *scylladbIterator) Scan(dest ...any) bool {
	return s.iter.Scan(dest...)
}

// NumRows returns a number of rows.
func (s *scylladbIterator) NumRows() int {
	return s.iter.NumRows()
}

// scylladbQuery implements query interface.
type scylladbQuery struct {
	query *gocql.Query
}

// Exec performs a ScyllaDB Query Exec.
func (s *scylladbQuery) Exec() error {
	return s.query.Exec()
}

// Iter returns a ScyllaDB iterator.
func (s *scylladbQuery) Iter() iterator {
	iter := scylladbIterator{iter: s.query.Iter()}

	return &iter
}

// MapScanCAS checks a ScyllaDB query an IF clause and scans the existing data into map[string]any (if any).
// This method wraps the `MapScanCAS` method of the underlying `query` object.
func (s *scylladbQuery) MapScanCAS(dest map[string]any) (applied bool, err error) {
	return s.query.MapScanCAS(dest)
}

// ScanCAS checks a ScyllaDB query with a IF clause and scans the existing data.
// This method wraps the `ScanCAS` method of the underlying `query` object.
func (s *scylladbQuery) ScanCAS(dest ...any) (applied bool, err error) {
	return s.query.ScanCAS(dest)
}

// scyllaClusterConfig implements clusterConfig.
type scyllaClusterConfig struct {
	clusterConfig *gocql.ClusterConfig
}

func newClusterConfig(config *Config) clusterConfig {
	var s scyllaClusterConfig

	config.Host = strings.TrimSuffix(strings.TrimSpace(config.Host), ",")
	hosts := strings.Split(config.Host, ",")
	s.clusterConfig = gocql.NewCluster(hosts...)
	s.clusterConfig.Keyspace = config.Keyspace
	s.clusterConfig.Port = config.Port
	s.clusterConfig.Authenticator = gocql.PasswordAuthenticator{Username: config.Username, Password: config.Password}

	return &s
}

// createSession creates a ScyllaDB session based on the provided configuration.
func (s *scyllaClusterConfig) createSession() (session, error) {
	sess, err := s.clusterConfig.CreateSession()
	if err != nil {
		return nil, err
	}

	return &scyllaSession{session: sess}, nil
}

// scyllaSession implements session.
type scyllaSession struct {
	session *gocql.Session
}

// Query creates a ScyllaDB query.
func (s *scyllaSession) Query(stmt string, values ...any) query {
	return &scylladbQuery{query: s.session.Query(stmt, values...)}
}

// newBatch creates a `gocql.BatchType`.
func (s *scyllaSession) newBatch(batchType gocql.BatchType) batch {
	return &scyllaBatch{batch: s.session.NewBatch(batchType)}
}

// executeBatch executes a batch operation.
func (s *scyllaSession) executeBatch(b batch) error {
	gocqlBatch := b.getBatch()

	return s.session.ExecuteBatch(gocqlBatch)
}

// executeBatchCAS executes a batch operation and returns true if successful.
func (s *scyllaSession) executeBatchCAS(batch batch, dest ...any) (bool, error) {
	gocqlBatch := batch.getBatch()

	applied, _, err := s.session.ExecuteBatchCAS(gocqlBatch, dest...)

	return applied, err
}

// scyllaBatch  implements batch.
type scyllaBatch struct {
	batch *gocql.Batch
}

// Query adds the query to the batch operation.
func (s *scyllaBatch) Query(stmt string, args ...any) {
	s.batch.Query(stmt, args...)
}

// getBatch returns the underlying `gocql.Batch`.
func (s *scyllaBatch) getBatch() *gocql.Batch {
	return s.batch
}

// getFields returns a slice of field pointers from the struct, mapping columns to their corresponding fields.
func (Client) getFields(columns []string, fieldNameIndex map[string]int, v reflect.Value) []any {
	fields := make([]any, len(columns))

	for i, column := range columns {
		if index, ok := fieldNameIndex[column]; ok {
			fields[i] = v.Field(index).Addr().Interface()
		} else {
			fields[i] = new(any)
		}
	}

	return fields
}

// addTrace starts a new trace span for the specified method and query.
func (c *Client) addTrace(ctx context.Context, method, query string) trace.Span {
	if c.tracer == nil {
		return nil
	}

	_, span := c.tracer.Start(ctx, fmt.Sprintf("scylladb-%v", method))

	span.SetAttributes(
		attribute.String("scylladb.query", query),
		attribute.String("scylladb.keyspace", c.config.Keyspace),
	)

	return span
}

// getColumnsFromColumnsInfo Extracts and returns a slice of column names from the provided gocql.ColumnInfo slice.
func (*Client) getColumnsFromColumnsInfo(columns []gocql.ColumnInfo) []string {
	cols := make([]string, 0)

	for _, column := range columns {
		cols = append(cols, column.Name)
	}

	return cols
}

// rowsToStruct Scans the iterator row data and maps it to the fields of the provided struct.
func (c *Client) rowsToStruct(iter iterator, vo reflect.Value) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	columns := c.getColumnsFromColumnsInfo(iter.Columns())
	fieldNameIndex := c.getFieldNameIndex(v)
	fields := c.getFields(columns, fieldNameIndex, v)

	_ = iter.Scan(fields...)

	if vo.CanSet() {
		vo.Set(v)
	}
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
			name = strcase.SnakeCase(f.Name)
		}

		fieldNameIndex[name] = i
	}

	return fieldNameIndex
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

	c.metrics.RecordHistogram(context.Background(), "app_scylla_stats", float64(duration), "hostname",
		c.config.Host, "keyspace", c.config.Keyspace)

	c.scylla.query = nil
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
