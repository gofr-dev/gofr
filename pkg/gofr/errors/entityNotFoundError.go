package errors

import (
	"fmt"
	"net/http"
)

// EntityNotFoundError represents an error for when an entity is not found in the system.
type EntityNotFoundError struct {
	FieldName  string
	FieldValue string
}

// Error returns the formatted error message.
func (e *EntityNotFoundError) Error() string {
	// for ex: "No entity found with id : 2"
	return fmt.Sprintf("No entity found with %s : %s", e.FieldName, e.FieldValue)
}

func (e *EntityNotFoundError) StatusCode() int {
	return http.StatusNotFound
}
