package cassandra

import (
	"errors"
	"fmt"
)

const (
	msgDestinationIsNotPointer = "destination is not pointer"
	msgUnexpectedMap           = "a map was not expected"
	msgUnsupportedBatchType    = "batch type not supported"
	msgBatchNotInitialised     = "batch not initialized"
)

var (
	errDestinationIsNotPointer = errors.New(msgDestinationIsNotPointer)
	errUnexpectedMap           = errors.New(msgUnexpectedMap)
	errUnsupportedBatchType    = errors.New(msgUnsupportedBatchType)
	errBatchNotInitialised     = errors.New(msgBatchNotInitialised)
)

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
