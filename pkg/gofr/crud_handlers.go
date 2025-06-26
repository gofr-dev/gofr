package gofr

import (
	"errors"
	"fmt"
	"reflect"

	"gofr.dev/pkg/gofr/datasource/sql"
)

var (
	errInvalidObject     = errors.New("unexpected object given for AddRESTHandlers")
	errEntityNotFound    = errors.New("entity not found")
	errObjectIsNil       = errors.New("object given for AddRESTHandlers is nil")
	errNonPointerObject  = errors.New("passed object is not pointer")
	errFieldCannotBeNull = errors.New("field cannot be null")
	errInvalidSQLTag     = errors.New("invalid sql tag")
)

type Create interface {
	Create(c *Context) (any, error)
}

type GetAll interface {
	GetAll(c *Context) (any, error)
}

type Get interface {
	Get(c *Context) (any, error)
}

type Update interface {
	Update(c *Context) (any, error)
}

type Delete interface {
	Delete(c *Context) (any, error)
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
	name        string
	entityType  reflect.Type
	primaryKey  string
	tableName   string
	restPath    string
	constraints map[string]sql.FieldConstraints
}

// scanEntity extracts entity information for CRUD operations.
func scanEntity(object any) (*entity, error) {
	if object == nil {
		return nil, errObjectIsNil
	}

	objType := reflect.TypeOf(object)
	if objType.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("failed to register routes for '%s' struct, %w", objType.Name(), errNonPointerObject)
	}

	entityType := objType.Elem()
	if entityType.Kind() != reflect.Struct {
		return nil, errInvalidObject
	}

	structName := entityType.Name()

	entityValue := reflect.ValueOf(object).Elem().Type()
	primaryKeyField := entityValue.Field(0) // Assume the first field is the primary key
	primaryKeyFieldName := toSnakeCase(primaryKeyField.Name)

	tableName := getTableName(object, structName)
	restPath := getRestPath(object, structName)

	e := &entity{
		name:        structName,
		entityType:  entityType,
		primaryKey:  primaryKeyFieldName,
		tableName:   tableName,
		restPath:    restPath,
		constraints: make(map[string]sql.FieldConstraints),
	}

	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		fieldName := toSnakeCase(field.Name)

		constraints, err := parseSQLTag(field.Tag)
		if err != nil {
			return nil, err
		}

		e.constraints[fieldName] = constraints
	}

	return e, nil
}

// registerCRUDHandlers registers CRUD handlers for an entity.
func (a *App) registerCRUDHandlers(e *entity, object any) {
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

func (e *entity) Create(c *Context) (any, error) {
	newEntity, err := e.bindAndValidateEntity(c)
	if err != nil {
		return nil, err
	}

	fieldNames, fieldValues := e.extractFields(newEntity)

	stmt, err := sql.InsertQuery(c.SQL.Dialect(), e.tableName, fieldNames, fieldValues, e.constraints)
	if err != nil {
		return nil, err
	}

	result, err := c.SQL.ExecContext(c, stmt, fieldValues...)
	if err != nil {
		return nil, err
	}

	var lastID any

	if hasAutoIncrementID(e.constraints) { // Check for auto-increment ID
		lastID, err = result.LastInsertId()
		if err != nil {
			return nil, err
		}
	} else {
		lastID = fieldValues[0]
	}

	return fmt.Sprintf("%s successfully created with id: %v", e.name, lastID), nil
}

func (e *entity) bindAndValidateEntity(c *Context) (any, error) {
	newEntity := reflect.New(e.entityType).Interface()

	err := c.Bind(newEntity)
	if err != nil {
		return nil, err
	}

	for i := 0; i < e.entityType.NumField(); i++ {
		field := e.entityType.Field(i)
		fieldName := toSnakeCase(field.Name)

		if e.constraints[fieldName].NotNull && reflect.ValueOf(newEntity).Elem().Field(i).Interface() == nil {
			return nil, fmt.Errorf("%w: %s", errFieldCannotBeNull, fieldName)
		}
	}

	return newEntity, nil
}

func (e *entity) extractFields(newEntity any) (fieldNames []string, fieldValues []any) {
	fieldNames = make([]string, 0, e.entityType.NumField())
	fieldValues = make([]any, 0, e.entityType.NumField())

	for i := 0; i < e.entityType.NumField(); i++ {
		field := e.entityType.Field(i)
		fieldName := toSnakeCase(field.Name)

		if e.constraints[fieldName].AutoIncrement {
			continue // Skip auto-increment fields for insertion
		}

		fieldNames = append(fieldNames, fieldName)
		fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
	}

	return fieldNames, fieldValues
}

func (e *entity) GetAll(c *Context) (any, error) {
	query := sql.SelectQuery(c.SQL.Dialect(), e.tableName)

	rows, err := c.SQL.QueryContext(c, query)
	if err != nil || rows.Err() != nil {
		return nil, err
	}

	defer rows.Close()

	dest := make([]any, e.entityType.NumField())
	val := reflect.New(e.entityType).Elem()

	for i := 0; i < e.entityType.NumField(); i++ {
		dest[i] = val.Field(i).Addr().Interface()
	}

	var entities []any

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

func (e *entity) Get(c *Context) (any, error) {
	newEntity := reflect.New(e.entityType).Interface()
	id := c.Request.PathParam("id")

	query := sql.SelectByQuery(c.SQL.Dialect(), e.tableName, e.primaryKey)

	row := c.SQL.QueryRowContext(c, query, id)

	dest := make([]any, e.entityType.NumField())
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

func (e *entity) Update(c *Context) (any, error) {
	newEntity := reflect.New(e.entityType).Interface()
	id := c.PathParam(e.primaryKey)

	err := c.Bind(newEntity)
	if err != nil {
		return nil, err
	}

	fieldNames := make([]string, 0, e.entityType.NumField())
	fieldValues := make([]any, 0, e.entityType.NumField())

	for i := 0; i < e.entityType.NumField(); i++ {
		field := e.entityType.Field(i)

		fieldNames = append(fieldNames, toSnakeCase(field.Name))
		fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
	}

	stmt := sql.UpdateByQuery(c.SQL.Dialect(), e.tableName, fieldNames[1:], e.primaryKey)

	_, err = c.SQL.ExecContext(c, stmt, append(fieldValues[1:], id)...)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s successfully updated with id: %s", e.name, id), nil
}

func (e *entity) Delete(c *Context) (any, error) {
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
