package scylladb

import (
	"errors"
	"fmt"
)

var (
	errUnsupportedBatchType    = errors.New("batch type not supported")
	errDestinationIsNotPointer = errors.New("destination is not pointer")
	errBatchNotInitialised     = errors.New("batch not initialized")
	errUnexpectedMap           = errors.New("a map was not expected")
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
