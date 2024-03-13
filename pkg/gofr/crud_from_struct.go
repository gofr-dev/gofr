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

type entityInfo struct {
	entityType          reflect.Type
	entityName          string
	primaryKeyFieldName string
}

func (a *App, ) registerCRUDHandlers(handlers CRUDHandlers, structName, primaryKeyFieldName string) {
	if handlers.GetAll != nil {
		a.GET(fmt.Sprintf("/%s", structName), handlers.GetAll)
	} else {
		a.GET(fmt.Sprintf("/%s", structName), defaultGetAllHandler)
	}

	if handlers.GetByID != nil {
		a.GET(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), handlers.GetByID)
	} else {
		a.GET(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), defaultGetHandler)
	}

	if handlers.Post != nil {
		a.POST(fmt.Sprintf("/%s", structName), handlers.Post)
	} else {
		a.POST(fmt.Sprintf("/%s", structName), defaultPostHandler)
	}

	if handlers.Put != nil {
		a.PUT(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), handlers.Put)
	} else {
		a.PUT(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), defaultPutHandler)
	}

	if handlers.Delete != nil {
		a.DELETE(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), handlers.Delete)
	} else {
		a.DELETE(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), defaultDeleteHandler)
	}
}

func defaultGetAllHandler(c *Context) (interface{}, error) {
	return nil, nil
}

func defaultGetHandler(c *Context) (interface{}, error) {
	entityType := c.Value("entityType").(reflect.Type)
	structName := c.Value("structName").(string)
	primaryKeyFieldName := c.Value("primaryKeyFieldName").(string)

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
func defaultPostHandler(c *Context) (interface{}, error) {
	entityType := c.Value("entityType").(reflect.Type)
	structName := c.Value("structName").(string)

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
func defaultPutHandler(c *Context) (interface{}, error) {
	return nil, nil
}
func defaultDeleteHandler(c *Context) (interface{}, error) {
	structName := c.Value("structName").(string)
	primaryKeyFieldName := c.Value("primaryKeyFieldName").(string)

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
