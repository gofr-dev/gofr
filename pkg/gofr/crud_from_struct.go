package gofr

import (
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

type EntityNotFound struct{}

func (e EntityNotFound) Error() string {
	return "entity not found!"
}

type entityConfig struct {
	name       string
	entityType reflect.Type
	primaryKey string
}

func scanEntity(entity interface{}) (*entityConfig, error) {
	entityType := reflect.TypeOf(entity)
	if entityType.Kind() != reflect.Struct {
		return nil, invalidType{}
	}

	structName := entityType.Name()

	// Assume the first field is the primary key
	primaryKeyField := entityType.Field(0)
	primaryKeyFieldName := strings.ToLower(primaryKeyField.Name)

	return &entityConfig{
		name:       structName,
		entityType: entityType,
		primaryKey: primaryKeyFieldName,
	}, nil
}

func (a *App) registerCRUDHandlers(handlers CRUDHandlers, ec entityConfig) {
	if handlers.GetAll != nil {
		a.GET(fmt.Sprintf("/%s", ec.name), handlers.GetAll)
	} else {
		a.GET(fmt.Sprintf("/%s", ec.name), defaultGetAllHandler(ec.entityType, ec.name))
	}

	if handlers.GetByID != nil {
		a.GET(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey), handlers.GetByID)
	} else {
		a.GET(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey),
			defaultGetHandler(ec.entityType, ec.name, ec.primaryKey))
	}

	if handlers.Post != nil {
		a.POST(fmt.Sprintf("/%s", ec.name), handlers.Post)
	} else {
		a.POST(fmt.Sprintf("/%s", ec.name), defaultPostHandler(ec.entityType, ec.name, ec.primaryKey))
	}

	if handlers.Put != nil {
		a.PUT(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey), handlers.Put)
	} else {
		a.PUT(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey),
			defaultPutHandler(ec.entityType, ec.name, ec.primaryKey))
	}

	if handlers.Delete != nil {
		a.DELETE(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey), handlers.Delete)
	} else {
		a.DELETE(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey),
			defaultDeleteHandler(ec.name, ec.primaryKey))
	}
}

func defaultGetAllHandler(entityType reflect.Type, structName string) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		newEntity := reflect.New(entityType).Interface()
		query := fmt.Sprintf("SELECT * FROM %s", structName)

		rows, err := c.SQL.QueryContext(c, query)
		if err != nil || rows.Err() != nil {
			return nil, err
		}

		defer rows.Close()

		// Create a slice of pointers to the struct's fields
		dest := make([]interface{}, entityType.NumField())
		val := reflect.ValueOf(newEntity).Elem()

		for i := 0; i < val.NumField(); i++ {
			dest[i] = val.Field(i).Addr().Interface()
		}

		var resp []interface{}

		// Scan the result into the struct's fields
		for rows.Next() {
			err = rows.Scan(dest...)
			if err != nil {
				return nil, err
			}

			resp = append(resp, newEntity)
		}

		c.Logf("GET ALL %s", structName)

		return resp, nil
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

		id := c.PathParam("id")

		var paramsList []string
		for i := 1; i < len(fieldNames); i++ {
			paramsList = append(paramsList, fmt.Sprintf("%s=?", fieldNames[i]))
		}

		query := strings.Join(paramsList, ", ")

		stmt := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s",
			structName,
			query,
			primaryKeyFieldName,
			id,
		)

		_, err = c.SQL.ExecContext(c, stmt, fieldValues[1:]...)
		if err != nil {
			return nil, err
		}

		c.Logf("PUT %s", structName)

		return fmt.Sprintf("PUT %s by %v", structName, primaryKeyFieldName), nil
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
			return nil, EntityNotFound{}
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
