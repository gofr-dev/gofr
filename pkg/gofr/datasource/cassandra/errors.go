package cassandra

import "fmt"

const (
	destinationIsNotPointer = "destination is not pointer"
	unexpectedMap           = "a map was not expected."
)

type DestinationIsNotPointer struct{}

func (d DestinationIsNotPointer) Error() string {
	return destinationIsNotPointer
}

type UnexpectedPointer struct {
	target string
}

func (d UnexpectedPointer) Error() string {
	return fmt.Sprintf("a pointer to %v was not expected.", d.target)
}

type UnexpectedSlice struct {
	target string
}

func (d UnexpectedSlice) Error() string {
	return fmt.Sprintf("a slice of %v was not expected.", d.target)
}

type UnexpectedMap struct{}

func (d UnexpectedMap) Error() string {
	return unexpectedMap
}
