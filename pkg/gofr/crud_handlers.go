package gofr

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	errInvalidResource = errors.New("unexpected resource given for CRUDFromStruct")
	errEntityNotFound  = errors.New("entity not found")
)

// EntityNotFound is an error type for indicating when an entity is not found.
type EntityNotFound struct{}

func (e EntityNotFound) Error() string {
	return "entity not found!"
}

type Create interface {
	Create(c *Context) (interface{}, error)
}

type GetAll interface {
	GetAll(c *Context) (interface{}, error)
}

type Get interface {
	Get(c *Context) (interface{}, error)
}

type Update interface {
	Update(c *Context) (interface{}, error)
}

type Delete interface {
	Delete(c *Context) (interface{}, error)
}

type CRUD interface {
	Create
	GetAll
	Get
	Update
	Delete
}

// entity stores information about an entity.
type entity struct {
	name       string
	entityType reflect.Type
	primaryKey string
}

// scanEntity extracts entity information for CRUD operations.
func scanEntity(resource interface{}) (*entity, error) {
	entityType := reflect.TypeOf(resource).Elem()
	if entityType.Kind() != reflect.Struct {
		return nil, errInvalidResource
	}

	structName := entityType.Name()

	entityValue := reflect.ValueOf(resource).Elem().Type()
	primaryKeyField := entityValue.Field(0) // Assume the first field is the primary key
	primaryKeyFieldName := strings.ToLower(primaryKeyField.Name)

	return &entity{
		name:       structName,
		entityType: entityType,
		primaryKey: primaryKeyFieldName,
	}, nil
}

// registerCRUDHandlers registers CRUD handlers for an entity.
func (a *App) registerCRUDHandlers(e entity, resource interface{}) {
	if fn, ok := resource.(Create); ok {
		a.POST(fmt.Sprintf("/%s", e.name), fn.Create)
	} else {
		a.POST(fmt.Sprintf("/%s", e.name), e.Create)
	}

	if fn, ok := resource.(GetAll); ok {
		a.GET(fmt.Sprintf("/%s", e.name), fn.GetAll)
	} else {
		a.GET(fmt.Sprintf("/%s", e.name), e.GetAll)
	}

	if fn, ok := resource.(Get); ok {
		a.GET(fmt.Sprintf("/%s/{%s}", e.name, e.primaryKey), fn.Get)
	} else {
		a.GET(fmt.Sprintf("/%s/{%s}", e.name, e.primaryKey), e.Get)
	}

	if fn, ok := resource.(Update); ok {
		a.PUT(fmt.Sprintf("/%s/{%s}", e.name, e.primaryKey), fn.Update)
	} else {
		a.PUT(fmt.Sprintf("/%s/{%s}", e.name, e.primaryKey), e.Update)
	}

	if fn, ok := resource.(Delete); ok {
		a.DELETE(fmt.Sprintf("/%s/{%s}", e.name, e.primaryKey), fn.Delete)
	} else {
		a.DELETE(fmt.Sprintf("/%s/{%s}", e.name, e.primaryKey), e.Delete)
	}
}

func (e *entity) Create(c *Context) (interface{}, error) {
	newEntity := reflect.New(e.entityType).Interface()
	err := c.Bind(newEntity)

	if err != nil {
		return nil, err
	}

	fieldNames := make([]string, 0, e.entityType.NumField())
	fieldValues := make([]interface{}, 0, e.entityType.NumField())

	for i := 0; i < e.entityType.NumField(); i++ {
		field := e.entityType.Field(i)
		fieldNames = append(fieldNames, field.Name)
		fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
	}

	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		e.name,
		strings.Join(fieldNames, ", "),
		strings.Repeat("?, ", len(fieldNames)-1)+"?",
	)

	_, err = c.SQL.ExecContext(c, stmt, fieldValues...)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s successfully created with id: %d", e.name, fieldValues[0]), nil
}

func (e *entity) GetAll(c *Context) (interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s", e.name)

	rows, err := c.SQL.QueryContext(c, query)
	if err != nil || rows.Err() != nil {
		return nil, err
	}

	defer rows.Close()

	dest := make([]interface{}, e.entityType.NumField())
	val := reflect.New(e.entityType).Elem()

	for i := 0; i < e.entityType.NumField(); i++ {
		dest[i] = val.Field(i).Addr().Interface()
	}

	var entities []interface{}

	for rows.Next() {
		newEntity := reflect.New(e.entityType).Interface()
		newVal := reflect.ValueOf(newEntity).Elem()

		err = rows.Scan(dest...)
		if err != nil {
			return nil, err
		}

		for i := 0; i < e.entityType.NumField(); i++ {
			scanVal := reflect.ValueOf(dest[i]).Elem().Interface()
			newVal.Field(i).Set(reflect.ValueOf(scanVal))
		}

		entities = append(entities, newEntity)
	}

	return entities, nil
}

func (e *entity) Get(c *Context) (interface{}, error) {
	newEntity := reflect.New(e.entityType).Interface()
	id := c.Request.PathParam("id")
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", e.name, e.primaryKey)
	row := c.SQL.QueryRowContext(c, query, id)

	dest := make([]interface{}, e.entityType.NumField())
	val := reflect.ValueOf(newEntity).Elem()

	for i := 0; i < val.NumField(); i++ {
		dest[i] = val.Field(i).Addr().Interface()
	}

	err := row.Scan(dest...)
	if err != nil {
		return nil, err
	}

	return newEntity, nil
}

func (e *entity) Update(c *Context) (interface{}, error) {
	newEntity := reflect.New(e.entityType).Interface()

	err := c.Bind(newEntity)
	if err != nil {
		return nil, err
	}

	fieldNames := make([]string, 0, e.entityType.NumField())
	fieldValues := make([]interface{}, 0, e.entityType.NumField())

	for i := 0; i < e.entityType.NumField(); i++ {
		field := e.entityType.Field(i)

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
		e.name,
		query,
		e.primaryKey,
		id,
	)

	_, err = c.SQL.ExecContext(c, stmt, fieldValues[1:]...)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s successfully updated with id: %s", e.name, id), nil
}

func (e *entity) Delete(c *Context) (interface{}, error) {
	id := c.PathParam("id")
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", e.name, e.primaryKey)

	result, err := c.SQL.ExecContext(c, query, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, errEntityNotFound
	}

	return fmt.Sprintf("%s successfully deleted with id: %v", e.name, id), nil
}
