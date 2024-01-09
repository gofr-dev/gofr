package container

import (
	"context"
	"database/sql"
	"reflect"
	"regexp"
	"strings"

	"gofr.dev/pkg/gofr/logging"
)

// DB is a wrapper around sql.DB which provides some more features.
type DB struct {
	*sql.DB
	logger logging.Logger
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
		d.logger.Error("We did not get a pointer. data is not settable.")

		return
	}

	// Deference the pointer to the underlying element, if the underlying element is a slice, multiple rows are expected.
	// If the underlying element is a struct, one row is expected.
	rv := rvo.Elem()

	switch rv.Kind() {
	case reflect.Slice:
		rows, err := d.QueryContext(ctx, query, args...)
		if err != nil {
			d.logger.Errorf("Error running query : %v", err)

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
