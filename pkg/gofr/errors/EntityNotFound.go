package errors

import "fmt"

// EntityNotFound is used when a given entity is not found in the system.
type EntityNotFound struct {
	Entity string `json:"entity"`
	ID     string `json:"id"`
}

// Error returns an error message indicating that the entity was not found.
func (e EntityNotFound) Error() string {
	return fmt.Sprintf("No '%v' found for Id: '%v'", e.Entity, e.ID)
}
