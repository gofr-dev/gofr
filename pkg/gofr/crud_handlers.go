package gofr

import (
	"fmt"
	"reflect"
	"strings"
)

type CRUD interface {
	Create(c *Context) (interface{}, error)
	GetAll(c *Context) (interface{}, error)
	Get(c *Context) (interface{}, error)
	Update(c *Context) (interface{}, error)
	Delete(c *Context) (interface{}, error)
}

// CRUDHandlers defines the interface for CRUD operations.
type handlers struct {
	CRUD

	config *entityConfig
}

// entityConfig stores information about an entity.
type entityConfig struct {
	name       string
	entityType reflect.Type
	primaryKey string
}

// scanEntity extracts entity information for CRUD operations.
func scanEntity(entity CRUD) (*entityConfig, error) {
	entityType := reflect.TypeOf(entity).Elem()
	structName := entityType.Name()

	entityValue := reflect.ValueOf(entity).Elem().Type()
	primaryKeyField := entityValue.Field(0) // Assume the first field is the primary key
	primaryKeyFieldName := strings.ToLower(primaryKeyField.Name)

	return &entityConfig{
		name:       structName,
		entityType: entityType,
		primaryKey: primaryKeyFieldName,
	}, nil
}

// registerCRUDHandlers registers CRUD handlers for an entity.
func (a *App) registerCRUDHandlers(h handlers) {
	a.POST(fmt.Sprintf("/%s", h.config.name), h.Create)

	a.GET(fmt.Sprintf("/%s", h.config.name), h.GetAll)

	a.GET(fmt.Sprintf("/%s/{%s}", h.config.name, h.config.primaryKey), h.Get)

	a.PUT(fmt.Sprintf("/%s/{%s}", h.config.name, h.config.primaryKey), h.Update)

	a.DELETE(fmt.Sprintf("/%s/{%s}", h.config.name, h.config.primaryKey), h.Delete)
}

func (h *handlers) Create(c *Context) (interface{}, error) {
	newEntity := reflect.New(h.config.entityType).Interface()
	err := c.Bind(newEntity)

	if err != nil {
		return nil, err
	}

	fieldNames := make([]string, 0, h.config.entityType.NumField())
	fieldValues := make([]interface{}, 0, h.config.entityType.NumField())

	for i := 0; i < h.config.entityType.NumField()-1; i++ {
		field := h.config.entityType.Field(i)
		fieldNames = append(fieldNames, field.Name)
		fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
	}

	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		h.config.name,
		strings.Join(fieldNames, ", "),
		strings.Repeat("?, ", len(fieldNames)-1)+"?",
	)

	_, err = c.SQL.ExecContext(c, stmt, fieldValues...)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s successfully created with id: %d", h.config.name, fieldValues[0]), nil
}

func (h *handlers) GetAll(c *Context) (interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s", h.config.name)

	rows, err := c.SQL.QueryContext(c, query)
	if err != nil || rows.Err() != nil {
		return nil, err
	}

	defer rows.Close()

	// Create a slice of pointers to the struct's fields
	dest := make([]interface{}, h.config.entityType.NumField()-1)
	val := reflect.New(h.config.entityType).Elem()

	for i := 0; i < h.config.entityType.NumField()-1; i++ {
		dest[i] = val.Field(i).Addr().Interface()
	}

	var entities []interface{}

	// Scan the result into the struct's fields
	for rows.Next() {
		// Reset newEntity for each row
		newEntity := reflect.New(h.config.entityType).Interface()
		newVal := reflect.ValueOf(newEntity).Elem() // Get Elem of newEntity

		// Scan the result into the struct's fields
		err = rows.Scan(dest...)
		if err != nil {
			return nil, err
		}

		// Set struct field values using reflection (consider type safety)
		for i := 0; i < h.config.entityType.NumField()-1; i++ {
			scanVal := reflect.ValueOf(dest[i]).Elem().Interface()
			newVal.Field(i).Set(reflect.ValueOf(scanVal))
		}

		// Append the entity to the list
		entities = append(entities, newEntity)
	}

	return entities, nil
}

func (h *handlers) Get(c *Context) (interface{}, error) {
	newEntity := reflect.New(h.config.entityType).Interface()
	// Implement logic to fetch entity by ID to fetch entity from database based on ID
	id := c.Request.PathParam("id")
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", h.config.name, h.config.primaryKey)

	row := c.SQL.QueryRowContext(c, query, id)

	// Create a slice of pointers to the struct's fields
	dest := make([]interface{}, h.config.entityType.NumField()-1)
	val := reflect.ValueOf(newEntity).Elem()

	for i := 0; i < val.NumField()-1; i++ {
		dest[i] = val.Field(i).Addr().Interface()
	}

	// Scan the result into the struct's fields
	err := row.Scan(dest...)
	if err != nil {
		return nil, err
	}

	return newEntity, nil
}

func (h *handlers) Update(c *Context) (interface{}, error) {
	newEntity := reflect.New(h.config.entityType).Interface()

	err := c.Bind(newEntity)
	if err != nil {
		return nil, err
	}

	fieldNames := make([]string, 0, h.config.entityType.NumField())
	fieldValues := make([]interface{}, 0, h.config.entityType.NumField())

	for i := 0; i < h.config.entityType.NumField(); i++ {
		field := h.config.entityType.Field(i)
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
		h.config.name,
		query,
		h.config.primaryKey,
		id,
	)

	_, err = c.SQL.ExecContext(c, stmt, fieldValues[1:]...)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("%s successfully updated with id: %s", h.config.name, id), nil
}

func (h *handlers) Delete(c *Context) (interface{}, error) {
	id := c.PathParam("id")
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", h.config.name, h.config.primaryKey)

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

	return fmt.Sprintf("%s successfully deleted with id: %v", h.config.name, id), nil
}

// EntityNotFound is an error type for indicating when an entity is not found.
type EntityNotFound struct{}

func (e EntityNotFound) Error() string {
	return "entity not found!"
}
