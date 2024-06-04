package cassandra

import "fmt"

type DestinationIsNotPointer struct{}

func (d DestinationIsNotPointer) Error() string {
	return fmt.Sprintf("destination is not pointer")
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
	return fmt.Sprintf("a map was not expected.")
}
