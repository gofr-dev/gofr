package gofr

import (
	"fmt"
	"reflect"
	"strings"
)

// CRUDHandlers defines the interface for CRUD operations.
type CRUDHandlers struct {
	GetAll  func(c *Context) (interface{}, error)
	GetByID func(c *Context) (interface{}, error)
	Post    func(c *Context) (interface{}, error)
	Put     func(c *Context) (interface{}, error)
	Delete  func(c *Context) (interface{}, error)
}

// EntityNotFound is an error type for indicating when an entity is not found.
type EntityNotFound struct{}

func (e EntityNotFound) Error() string {
	return "entity not found!"
}

// entityConfig stores information about an entity.
type entityConfig struct {
	name       string
	entityType reflect.Type
	primaryKey string
}

// scanEntity extracts entity information for CRUD operations.
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

// verifyHandlerSignature checks if the handler method signature is valid.
func verifyHandlerSignature(method *reflect.Method) bool {
	methodType := method.Func.Type()
	isContext := methodType.In(1) == reflect.TypeOf(&Context{})

	return method.Func.IsValid() && methodType.NumIn() == 2 && isContext && methodType.NumOut() == 2
}

// registerCRUDHandlers registers CRUD handlers for an entity.
func (a *App) registerCRUDHandlers(
	handlers CRUDHandlers, ec entityConfig) {
	if handlers.GetAll != nil {
		a.GET(fmt.Sprintf("/%s", ec.name), handlers.GetAll)
	} else {
		a.GET(fmt.Sprintf("/%s", ec.name), defaultGetAllHandler(ec))
	}

	if handlers.GetByID != nil {
		a.GET(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey), handlers.GetByID)
	} else {
		a.GET(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey),
			defaultGetHandler(ec))
	}

	if handlers.Post != nil {
		a.POST(fmt.Sprintf("/%s", ec.name), handlers.Post)
	} else {
		a.POST(fmt.Sprintf("/%s", ec.name), defaultPostHandler(ec))
	}

	if handlers.Put != nil {
		a.PUT(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey), handlers.Put)
	} else {
		a.PUT(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey),
			defaultPutHandler(ec))
	}

	if handlers.Delete != nil {
		a.DELETE(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey), handlers.Delete)
	} else {
		a.DELETE(fmt.Sprintf("/%s/{%s}", ec.name, ec.primaryKey),
			defaultDeleteHandler(ec))
	}
}

// defaultGetAllHandler is the default handler for the GetAll operation.
func defaultGetAllHandler(ec entityConfig) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		query := fmt.Sprintf("SELECT * FROM %s", ec.name)

		rows, err := c.SQL.QueryContext(c, query)
		if err != nil || rows.Err() != nil {
			return nil, err
		}

		defer rows.Close()

		// Create a slice of pointers to the struct's fields
		dest := make([]interface{}, ec.entityType.NumField())
		val := reflect.New(ec.entityType).Elem()

		for i := 0; i < ec.entityType.NumField(); i++ {
			dest[i] = val.Field(i).Addr().Interface()
		}

		var entities []interface{}

		// Scan the result into the struct's fields
		for rows.Next() {
			// Reset newEntity for each row
			newEntity := reflect.New(ec.entityType).Interface()
			newVal := reflect.ValueOf(newEntity).Elem() // Get Elem of newEntity

			// Scan the result into the struct's fields
			err = rows.Scan(dest...)
			if err != nil {
				return nil, err
			}

			// Set struct field values using reflection (consider type safety)
			for i := 0; i < ec.entityType.NumField(); i++ {
				scanVal := reflect.ValueOf(dest[i]).Elem().Interface()
				newVal.Field(i).Set(reflect.ValueOf(scanVal))
			}

			// Append the entity to the list
			entities = append(entities, newEntity)
		}

		c.Logf("GET ALL %s", ec.name)

		return entities, nil
	}
}

// defaultGetHandler is the default handler for the GetByID operation.
func defaultGetHandler(ec entityConfig) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		newEntity := reflect.New(ec.entityType).Interface()
		// Implement logic to fetch entity by ID to fetch entity from database based on ID
		id := c.Request.PathParam("id")
		query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", ec.name, ec.primaryKey)

		row := c.SQL.QueryRowContext(c, query, id)

		// Create a slice of pointers to the struct's fields
		dest := make([]interface{}, ec.entityType.NumField())
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

// defaultPostHandler is the default handler for the Post operation.
func defaultPostHandler(ec entityConfig) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		newEntity := reflect.New(ec.entityType).Interface()
		err := c.Bind(newEntity)

		if err != nil {
			return nil, err
		}

		fieldNames := make([]string, 0, ec.entityType.NumField())
		fieldValues := make([]interface{}, 0, ec.entityType.NumField())

		for i := 0; i < ec.entityType.NumField(); i++ {
			field := ec.entityType.Field(i)
			fieldNames = append(fieldNames, field.Name)
			fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
		}

		stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			ec.name,
			strings.Join(fieldNames, ", "),
			strings.Repeat("?, ", len(fieldNames)-1)+"?",
		)

		_, err = c.SQL.ExecContext(c, stmt, fieldValues...)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("%s successfully created with id: %d", ec.name, fieldValues[0]), nil
	}
}

// defaultPutHandler is the default handler for the Put operation.
func defaultPutHandler(ec entityConfig) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		newEntity := reflect.New(ec.entityType).Interface()

		err := c.Bind(newEntity)
		if err != nil {
			return nil, err
		}

		fieldNames := make([]string, 0, ec.entityType.NumField())
		fieldValues := make([]interface{}, 0, ec.entityType.NumField())

		for i := 0; i < ec.entityType.NumField(); i++ {
			field := ec.entityType.Field(i)
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
			ec.name,
			query,
			ec.primaryKey,
			id,
		)

		_, err = c.SQL.ExecContext(c, stmt, fieldValues[1:]...)
		if err != nil {
			return nil, err
		}

		c.Logf("PUT %s", ec.name)

		return fmt.Sprintf("PUT %s by %v", ec.name, ec.primaryKey), nil
	}
}

// defaultDeleteHandler is the default handler for the Delete operation.
func defaultDeleteHandler(ec entityConfig) func(c *Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		// Implement logic to delete entity by ID
		id := c.PathParam("id")
		query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", ec.name, ec.primaryKey)

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

		return fmt.Sprintf("%s successfully deleted with id : %v", ec.name, id), nil
	}
}

// wrapHandler wraps a handler function with the entity type as its first argument.
func wrapHandler(fn reflect.Value, entityType reflect.Type) func(*Context) (interface{}, error) {
	return func(c *Context) (interface{}, error) {
		entity := reflect.New(entityType).Elem().Interface()
		return fn.Call([]reflect.Value{reflect.ValueOf(entity), reflect.ValueOf(c)})[0].Interface(), nil
	}
}
