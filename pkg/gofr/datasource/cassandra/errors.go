package cassandra

import (
	"errors"
	"fmt"
)

var (
	ErrDestinationIsNotPointer = errors.New("destination is not pointer")
	ErrUnexpectedMap           = errors.New("a map was not expected")
	ErrUnsupportedBatchType    = errors.New("batch type not supported")
	ErrBatchNotInitialised     = errors.New("batch not initialized")
)

type ErrUnexpectedPointer struct {
	target string
}

func (d ErrUnexpectedPointer) Error() string {
	return fmt.Sprintf("a pointer to %v was not expected.", d.target)
}

type ErrUnexpectedSlice struct {
	target string
}

func (d ErrUnexpectedSlice) Error() string {
	return fmt.Sprintf("a slice of %v was not expected.", d.target)
}
