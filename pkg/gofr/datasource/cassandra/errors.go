package cassandra

import (
	"errors"
	"fmt"
)

var (
	errDestinationIsNotPointer = errors.New("destination is not pointer")
	errUnexpectedMap           = errors.New("a map was not expected")
	errUnsupportedBatchType    = errors.New("batch type not supported")
	errBatchNotInitialised     = errors.New("batch not initialized")
)

type errUnexpectedPointer struct {
	target string
}

func (d errUnexpectedPointer) Error() string {
	return fmt.Sprintf("a pointer to %v was not expected.", d.target)
}

type errUnexpectedSlice struct {
	target string
}

func (d errUnexpectedSlice) Error() string {
	return fmt.Sprintf("a slice of %v was not expected.", d.target)
}
