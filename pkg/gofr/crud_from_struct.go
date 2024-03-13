package gofr

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type CRUDHandlers struct {
	GetAll  func(c *Context) (interface{}, error)
	GetByID func(c *Context) (interface{}, error)
	Post    func(c *Context) (interface{}, error)
	Put     func(c *Context) (interface{}, error)
	Delete  func(c *Context) (interface{}, error)
}

func (a *App) registerCRUDHandlers(handlers CRUDHandlers, entityType reflect.Type, structName, primaryKeyFieldName string) {
	if handlers.GetAll != nil {
		a.GET(fmt.Sprintf("/%s", structName), handlers.GetAll)
	} else {
		a.GET(fmt.Sprintf("/%s", structName), defaultGetAllHandler(structName, primaryKeyFieldName))
	}

	if handlers.GetByID != nil {
		a.GET(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), handlers.GetByID)
	} else {
		a.GET(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName),
			defaultGetHandler(entityType, structName, primaryKeyFieldName))
	}

	if handlers.Post != nil {
		a.POST(fmt.Sprintf("/%s", structName), handlers.Post)
	} else {
		a.POST(fmt.Sprintf("/%s", structName), defaultPostHandler(entityType, structName, primaryKeyFieldName))
	}

	if handlers.Put != nil {
		a.PUT(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), handlers.Put)
	} else {
		a.PUT(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName),
			defaultPutHandler(entityType, structName, primaryKeyFieldName))
	}

	if handlers.Delete != nil {
		a.DELETE(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), handlers.Delete)
	} else {
		a.DELETE(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName),
			defaultDeleteHandler(structName, primaryKeyFieldName))
	}
}

func defaultGetAllHandler(structName, primaryKeyFieldName string) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		return nil, nil
	}
}

func defaultGetHandler(entityType reflect.Type, structName, primaryKeyFieldName string) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		newEntity := reflect.New(entityType).Interface()
		// Implement logic to fetch entity by ID to fetch entity from database based on ID
		id := c.Request.PathParam("id")
		query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", structName, primaryKeyFieldName)

		row := c.SQL.QueryRowContext(c, query, id)

		// Create a slice of pointers to the struct's fields
		dest := make([]interface{}, entityType.NumField())
		val := reflect.ValueOf(newEntity).Elem()

		for i := 0; i < val.NumField(); i++ {
			dest[i] = val.Field(i).Addr().Interface()
		}

		// Scan the result into the struct's fields
		err := row.Scan(dest...)
		if err != nil {
			return nil, err
		}

		return newEntity, nil
	}
}

func defaultPostHandler(entityType reflect.Type, structName, _ string) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		newEntity := reflect.New(entityType).Interface()
		err := c.Bind(newEntity)

		if err != nil {
			return nil, err
		}

		fieldNames := make([]string, 0, entityType.NumField())
		fieldValues := make([]interface{}, 0, entityType.NumField())

		for i := 0; i < entityType.NumField(); i++ {
			field := entityType.Field(i)
			fieldNames = append(fieldNames, field.Name)
			fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
		}

		stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			structName,
			strings.Join(fieldNames, ", "),
			strings.Repeat("?, ", len(fieldNames)-1)+"?",
		)

		_, err = c.SQL.ExecContext(c, stmt, fieldValues...)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("%s successfully created with id: %d", structName, fieldValues[0]), nil
	}
}

func defaultPutHandler(entityType reflect.Type, structName, primaryKeyFieldName string) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		return nil, nil
	}
}

func defaultDeleteHandler(structName, primaryKeyFieldName string) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		// Implement logic to delete entity by ID
		id := c.PathParam("id")
		query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", structName, primaryKeyFieldName)

		result, err := c.SQL.ExecContext(c, query, id)
		if err != nil {
			return nil, err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}

		if rowsAffected == 0 {
			return nil, errors.New("entity not found")
		}

		return fmt.Sprintf("%s successfully deleted with id : %v", structName, id), nil
	}
}

func wrapGetAll(fn reflect.Value, entityType reflect.Type) func(*Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		user := reflect.New(entityType).Elem().Interface()
		return fn.Call([]reflect.Value{reflect.ValueOf(user), reflect.ValueOf(c)})[0].Interface(), nil
	}
}

func wrapGet(fn reflect.Value, entityType reflect.Type) func(*Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		user := reflect.New(entityType).Elem().Interface()
		return fn.Call([]reflect.Value{reflect.ValueOf(user), reflect.ValueOf(c)})[0].Interface(), nil
	}
}

func wrapPost(fn reflect.Value, entityType reflect.Type) func(*Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		user := reflect.New(entityType).Elem().Interface()
		return fn.Call([]reflect.Value{reflect.ValueOf(user), reflect.ValueOf(c)})[0].Interface(), nil
	}
}

func wrapPut(fn reflect.Value, entityType reflect.Type) func(*Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		user := reflect.New(entityType).Elem().Interface()
		return fn.Call([]reflect.Value{reflect.ValueOf(user), reflect.ValueOf(c)})[0].Interface(), nil
	}
}

func wrapDelete(fn reflect.Value, entityType reflect.Type) func(*Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		user := reflect.New(entityType).Elem().Interface()
		return fn.Call([]reflect.Value{reflect.ValueOf(user), reflect.ValueOf(c)})[0].Interface(), nil
	}
}
