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
	Type     string        `json:"type"`
	Query    string        `json:"query"`
	Duration int64         `json:"duration"`
	Args     []interface{} `json:"args,omitempty"`
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

func (d *DB) logQuery(start time.Time, queryType, query string, args ...interface{}) {
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

	return words[0]
}

func (d *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	defer d.logQuery(time.Now(), "Query", query, args...)
	return d.DB.Query(query, args...)
}

func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	defer d.logQuery(time.Now(), "QueryContext", query, args...)
	return d.DB.QueryContext(ctx, query, args...)
}

func (d *DB) Dialect() string {
	return d.config.Dialect
}

func (d *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	defer d.logQuery(time.Now(), "QueryRow", query, args...)
	return d.DB.QueryRow(query, args...)
}

func (d *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	defer d.logQuery(time.Now(), "QueryRowContext", query, args...)
	return d.DB.QueryRowContext(ctx, query, args...)
}

func (d *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	defer d.logQuery(time.Now(), "Exec", query, args...)
	return d.DB.Exec(query, args...)
}

func (d *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	defer d.logQuery(time.Now(), "ExecContext", query, args...)
	return d.DB.ExecContext(ctx, query, args...)
}

func (d *DB) Prepare(query string) (*sql.Stmt, error) {
	defer d.logQuery(time.Now(), "Prepare", query)
	return d.DB.Prepare(query)
}

func (d *DB) Begin() (*Tx, error) {
	tx, err := d.DB.Begin()
	if err != nil {
		return nil, err
	}

	return &Tx{Tx: tx, config: d.config, logger: d.logger, metrics: d.metrics}, nil
}

type Tx struct {
	*sql.Tx
	config  *DBConfig
	logger  datasource.Logger
	metrics Metrics
}

func (t *Tx) logQuery(start time.Time, queryType, query string, args ...interface{}) {
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

func (t *Tx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	defer t.logQuery(time.Now(), "TxQuery", query, args...)
	return t.Tx.Query(query, args...)
}

func (t *Tx) QueryRow(query string, args ...interface{}) *sql.Row {
	defer t.logQuery(time.Now(), "TxQueryRow", query, args...)
	return t.Tx.QueryRow(query, args...)
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	defer t.logQuery(time.Now(), "TxQueryRowContext", query, args...)
	return t.Tx.QueryRowContext(ctx, query, args...)
}

func (t *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	defer t.logQuery(time.Now(), "TxExec", query, args...)
	return t.Tx.Exec(query, args...)
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	defer t.logQuery(time.Now(), "TxExecContext", query, args...)
	return t.Tx.ExecContext(ctx, query, args...)
}

func (t *Tx) Prepare(query string) (*sql.Stmt, error) {
	defer t.logQuery(time.Now(), "TxPrepare", query)
	return t.Tx.Prepare(query)
}

func (t *Tx) Commit() error {
	defer t.logQuery(time.Now(), "TxCommit", "COMMIT")
	return t.Tx.Commit()
}

func (t *Tx) Rollback() error {
	defer t.logQuery(time.Now(), "TxRollback", "ROLLBACK")
	return t.Tx.Rollback()
}

// Select runs a query with args and binds the result of the query to the data.
// data should ba a point to a slice, struct or any type. Slice will return multiple
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
func (d *DB) Select(ctx context.Context, data interface{}, query string, args ...interface{}) {
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

		if rvo.Elem().CanSet() {
			rvo.Elem().Set(rv)
		}

	case reflect.Struct:
		rows, _ := d.QueryContext(ctx, query, args...)
		for rows.Next() {
			d.rowsToStruct(rows, rv)
		}

	default:
		d.logger.Debugf("a pointer to %v was not expected.", rv.Kind().String())
	}
}

func (d *DB) rowsToStruct(rows *sql.Rows, vo reflect.Value) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	// Map fields and their indexes by normalised name
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

	fields := []interface{}{}
	columns, _ := rows.Columns()

	for _, c := range columns {
		if i, ok := fieldNameIndex[c]; ok {
			fields = append(fields, v.Field(i).Addr().Interface())
		} else {
			var i interface{}
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
