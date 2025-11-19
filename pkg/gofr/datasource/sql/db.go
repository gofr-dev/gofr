// Package sql provides functionalities to interact with SQL databases using the database/sql package.This package
// includes a wrapper around sql.DB and sql.Tx to provide additional features such as query logging, metrics recording,
// and error handling.
package sql

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

// DB is a wrapper around sql.DB which provides some more features.
type DB struct {
	// contains unexported or private fields
	*sql.DB
	logger  datasource.Logger
	config  *DBConfig
	metrics Metrics
}

type Log struct {
	Type     string `json:"type"`
	Query    string `json:"query"`
	Duration int64  `json:"duration"`
	Args     []any  `json:"args,omitempty"`
}

func (l *Log) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;24m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",
		l.Type, "SQL", l.Duration, clean(l.Query))
}

func clean(query string) string {
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	query = strings.TrimSpace(query)

	return query
}

func (d *DB) sendOperationStats(start time.Time, queryType, query string, args ...any) {
	duration := time.Since(start).Milliseconds()

	d.logger.Debug(&Log{
		Type:     queryType,
		Query:    query,
		Duration: duration,
		Args:     args,
	})

	d.metrics.RecordHistogram(context.Background(), "app_sql_stats", float64(duration), "hostname", d.config.HostName,
		"database", d.config.Database, "type", getOperationType(query))
}

func getOperationType(query string) string {
	query = strings.TrimSpace(query)
	words := strings.Split(query, " ")

	return strings.ToUpper(words[0])
}

func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	defer d.sendOperationStats(time.Now(), "Query", query, args...)
	return d.DB.QueryContext(context.Background(), query, args...)
}

func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	defer d.sendOperationStats(time.Now(), "QueryContext", query, args...)
	return d.DB.QueryContext(ctx, query, args...)
}

func (d *DB) Dialect() string {
	return d.config.Dialect
}

func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	defer d.sendOperationStats(time.Now(), "QueryRow", query, args...)
	return d.DB.QueryRowContext(context.Background(), query, args...)
}

func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	defer d.sendOperationStats(time.Now(), "QueryRowContext", query, args...)
	return d.DB.QueryRowContext(ctx, query, args...)
}

func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	defer d.sendOperationStats(time.Now(), "Exec", query, args...)
	return d.DB.ExecContext(context.Background(), query, args...)
}

func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	defer d.sendOperationStats(time.Now(), "ExecContext", query, args...)
	return d.DB.ExecContext(ctx, query, args...)
}

func (d *DB) Prepare(query string) (*sql.Stmt, error) {
	defer d.sendOperationStats(time.Now(), "Prepare", query)
	return d.DB.PrepareContext(context.Background(), query)
}

func (d *DB) Begin() (*Tx, error) {
	tx, err := d.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	return &Tx{Tx: tx, config: d.config, logger: d.logger, metrics: d.metrics}, nil
}

func (d *DB) Close() error {
	if d.DB != nil {
		return d.DB.Close()
	}

	return nil
}

type Tx struct {
	*sql.Tx
	config  *DBConfig
	logger  datasource.Logger
	metrics Metrics
}

func (t *Tx) sendOperationStats(start time.Time, queryType, query string, args ...any) {
	duration := time.Since(start).Milliseconds()

	t.logger.Debug(&Log{
		Type:     queryType,
		Query:    query,
		Duration: duration,
		Args:     args,
	})

	t.metrics.RecordHistogram(context.Background(), "app_sql_stats", float64(duration), "hostname", t.config.HostName,
		"database", t.config.Database, "type", getOperationType(query))
}

func (t *Tx) Query(query string, args ...any) (*sql.Rows, error) {
	defer t.sendOperationStats(time.Now(), "TxQuery", query, args...)
	return t.Tx.QueryContext(context.Background(), query, args...)
}

func (t *Tx) QueryRow(query string, args ...any) *sql.Row {
	defer t.sendOperationStats(time.Now(), "TxQueryRow", query, args...)
	return t.Tx.QueryRowContext(context.Background(), query, args...)
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	defer t.sendOperationStats(time.Now(), "TxQueryRowContext", query, args...)
	return t.Tx.QueryRowContext(ctx, query, args...)
}

func (t *Tx) Exec(query string, args ...any) (sql.Result, error) {
	defer t.sendOperationStats(time.Now(), "TxExec", query, args...)
	return t.Tx.ExecContext(context.Background(), query, args...)
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	defer t.sendOperationStats(time.Now(), "TxExecContext", query, args...)
	return t.Tx.ExecContext(ctx, query, args...)
}

func (t *Tx) Prepare(query string) (*sql.Stmt, error) {
	defer t.sendOperationStats(time.Now(), "TxPrepare", query)
	return t.Tx.PrepareContext(context.Background(), query)
}

func (t *Tx) Commit() error {
	defer t.sendOperationStats(time.Now(), "TxCommit", "COMMIT")
	return t.Tx.Commit()
}

func (t *Tx) Rollback() error {
	defer t.sendOperationStats(time.Now(), "TxRollback", "ROLLBACK")
	return t.Tx.Rollback()
}

// Select runs a query with args and binds the result of the query to the data.
// data should be a point to a slice, struct or any type. Slice will return multiple
// objects whereas struct will return a single object.
//
// Example Usages:
//
//  1. Get multiple rows with only one column
//     ids := make([]int, 0)
//     db.Select(ctx, &ids, "select id from users")
//
//  2. Get a single object from database
//     type user struct {
//     Name  string
//     ID    int
//     Image string
//     }
//     u := user{}
//     db.Select(ctx, &u, "select * from users where id=?", 1)
//
//  3. Get array of objects from multiple rows
//     type user struct {
//     Name  string
//     ID    int
//     Image string `db:"image_url"`
//     }
//     users := []user{}
//     db.Select(ctx, &users, "select * from users")
//
//nolint:exhaustive // We just want to take care of slice and struct in this case.
func (d *DB) Select(ctx context.Context, data any, query string, args ...any) {
	// If context is done, it is not needed
	if ctx.Err() != nil {
		return
	}

	// First confirm that what we got in v is a pointer else it won't be settable
	rvo := reflect.ValueOf(data)
	if rvo.Kind() != reflect.Ptr {
		d.logger.Error("we did not get a pointer. data is not settable.")
		return
	}

	// Deference the pointer to the underlying element, if the underlying element is a slice, multiple rows are expected.
	// If the underlying element is a struct, one row is expected.
	rv := rvo.Elem()

	switch rv.Kind() {
	case reflect.Slice:
		d.selectSlice(ctx, query, args, rvo, rv)

	case reflect.Struct:
		d.selectStruct(ctx, query, args, rv)

	default:
		d.logger.Debugf("a pointer to %v was not expected.", rv.Kind().String())
	}
}

func (d *DB) selectSlice(ctx context.Context, query string, args []any, rvo, rv reflect.Value) {
	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		d.logger.Errorf("error running query: %v", err)
		return
	}

	for rows.Next() {
		val := reflect.New(rv.Type().Elem())

		if rv.Type().Elem().Kind() == reflect.Struct {
			d.rowsToStruct(rows, val)
		} else {
			_ = rows.Scan(val.Interface())
		}

		rv = reflect.Append(rv, val.Elem())
	}

	if rows.Err() != nil {
		d.logger.Errorf("error parsing rows : %v", err)
		return
	}

	if rvo.Elem().CanSet() {
		rvo.Elem().Set(rv)
	}
}

func (d *DB) selectStruct(ctx context.Context, query string, args []any, rv reflect.Value) {
	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		d.logger.Errorf("error running query: %v", err)
		return
	}

	for rows.Next() {
		d.rowsToStruct(rows, rv)
	}

	if rows.Err() != nil {
		d.logger.Errorf("error parsing rows : %v", err)
		return
	}
}

func (*DB) rowsToStruct(rows *sql.Rows, vo reflect.Value) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	// Map fields and their indexes by normalized name
	fieldNameIndex := map[string]int{}

	for i := 0; i < v.Type().NumField(); i++ {
		var name string

		f := v.Type().Field(i)
		tag := f.Tag.Get("db")

		if tag != "" {
			name = tag
		} else {
			name = ToSnakeCase(f.Name)
		}

		fieldNameIndex[name] = i
	}

	fields := []any{}
	columns, _ := rows.Columns()

	for _, c := range columns {
		if i, ok := fieldNameIndex[c]; ok {
			fields = append(fields, v.Field(i).Addr().Interface())
		} else {
			var i any

			fields = append(fields, &i)
		}
	}

	_ = rows.Scan(fields...)

	if vo.CanSet() {
		vo.Set(v)
	}
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}
