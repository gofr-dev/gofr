package cassandra

import "fmt"

const (
	msgDestinationIsNotPointer = "destination is not pointer"
	msgUnexpectedMap           = "a map was not expected."
)

type destinationIsNotPointer struct{}

func (d destinationIsNotPointer) Error() string {
	return msgDestinationIsNotPointer
}

type unexpectedPointer struct {
	target string
}

func (d unexpectedPointer) Error() string {
	return fmt.Sprintf("a pointer to %v was not expected.", d.target)
}

type unexpectedSlice struct {
	target string
}

func (d unexpectedSlice) Error() string {
	return fmt.Sprintf("a slice of %v was not expected.", d.target)
}

type unexpectedMap struct{}

func (d unexpectedMap) Error() string {
	return msgUnexpectedMap
}
