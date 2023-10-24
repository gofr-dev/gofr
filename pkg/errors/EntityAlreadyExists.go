package errors

// EntityAlreadyExists is used when a given entity already exist in the system
type EntityAlreadyExists struct{}

// Error returns a message indicating that the entity already exists
func (e EntityAlreadyExists) Error() string {
	return "entity already exists"
}
