package errors

// EntityAlreadyExists is used when a given entity already exist in the system.
type EntityAlreadyExists struct{}

const errMessage = "entity already exists"

// Error returns a message indicating that the entity already exists.
func (e EntityAlreadyExists) Error() string {
	return errMessage
}
