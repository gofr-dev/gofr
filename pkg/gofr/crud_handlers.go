package gofr

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"gofr.dev/pkg/gofr/datasource/sql"
)

var (
	errInvalidObject  = errors.New("unexpected object given for AddRESTHandlers")
	errEntityNotFound = errors.New("entity not found")
)

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

type TableNameOverrider interface {
	TableName() string
}

type RestPathOverrider interface {
	RestPath() string
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
	tableName  string
	restPath   string
}

// scanEntity extracts entity information for CRUD operations.
func scanEntity(object interface{}) (*entity, error) {
	entityType := reflect.TypeOf(object).Elem()
	if entityType.Kind() != reflect.Struct {
		return nil, errInvalidObject
	}

	structName := entityType.Name()

	entityValue := reflect.ValueOf(object).Elem().Type()
	primaryKeyField := entityValue.Field(0) // Assume the first field is the primary key
	primaryKeyFieldName := toSnakeCase(primaryKeyField.Name)

	tableName := getTableName(object, structName)
	restPath := getRestPath(object, structName)

	return &entity{
		name:       structName,
		entityType: entityType,
		primaryKey: primaryKeyFieldName,
		tableName:  tableName,
		restPath:   restPath,
	}, nil
}

func getTableName(object any, structName string) string {
	if v, ok := object.(TableNameOverrider); ok {
		return v.TableName()
	}

	return toSnakeCase(structName)
}

func getRestPath(object any, structName string) string {
	if v, ok := object.(RestPathOverrider); ok {
		return v.RestPath()
	}

	return structName
}

// registerCRUDHandlers registers CRUD handlers for an entity.
func (a *App) registerCRUDHandlers(e *entity, object interface{}) {
	basePath := fmt.Sprintf("/%s", e.restPath)
	idPath := fmt.Sprintf("/%s/{%s}", e.restPath, e.primaryKey)

	if fn, ok := object.(Create); ok {
		a.POST(basePath, fn.Create)
	} else {
		a.POST(basePath, e.Create)
	}

	if fn, ok := object.(GetAll); ok {
		a.GET(basePath, fn.GetAll)
	} else {
		a.GET(basePath, e.GetAll)
	}

	if fn, ok := object.(Get); ok {
		a.GET(idPath, fn.Get)
	} else {
		a.GET(idPath, e.Get)
	}

	if fn, ok := object.(Update); ok {
		a.PUT(idPath, fn.Update)
	} else {
		a.PUT(idPath, e.Update)
	}

	if fn, ok := object.(Delete); ok {
		a.DELETE(idPath, fn.Delete)
	} else {
		a.DELETE(idPath, e.Delete)
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
		fieldNames = append(fieldNames, toSnakeCase(field.Name))
		fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
	}

	stmt := sql.InsertQuery(c.SQL.Dialect(), e.tableName, fieldNames)

	_, err = c.SQL.ExecContext(c, stmt, fieldValues...)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s successfully created with id: %d", e.name, fieldValues[0]), nil
}

func (e *entity) GetAll(c *Context) (interface{}, error) {
	query := sql.SelectQuery(c.SQL.Dialect(), e.tableName)

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

	query := sql.SelectByQuery(c.SQL.Dialect(), e.tableName, e.primaryKey)

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

		fieldNames = append(fieldNames, toSnakeCase(field.Name))
		fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
	}

	id := c.PathParam("id")

	stmt := sql.UpdateByQuery(c.SQL.Dialect(), e.tableName, fieldNames[1:], e.primaryKey)

	_, err = c.SQL.ExecContext(c, stmt, append(fieldValues[1:], fieldValues[0])...)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s successfully updated with id: %s", e.name, id), nil
}

func (e *entity) Delete(c *Context) (interface{}, error) {
	id := c.PathParam("id")

	query := sql.DeleteByQuery(c.SQL.Dialect(), e.tableName, e.primaryKey)

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

func toSnakeCase(str string) string {
	diff := 'a' - 'A'
	length := len(str)

	var builder strings.Builder

	for i, char := range str {
		if char >= 'a' {
			builder.WriteRune(char)
			continue
		}

		if (i != 0 || i == length-1) && ((i > 0 && rune(str[i-1]) >= 'a') || (i < length-1 && rune(str[i+1]) >= 'a')) {
			builder.WriteRune('_')
		}

		builder.WriteRune(char + diff)
	}

	return builder.String()
}
