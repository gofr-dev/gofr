package cassandra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DestinationIsNotPointer_Error(t *testing.T) {
	err := ErrDestinationIsNotPointer

	assert.Equal(t, err, ErrDestinationIsNotPointer)
}

func Test_UnexpectedPointer_Error(t *testing.T) {
	expected := "a pointer to int was not expected."
	err := UnexpectedPointer{target: "int"}

	assert.ErrorContains(t, err, expected)
}

func Test_UnexpectedSlice_Error(t *testing.T) {
	expected := "a slice of int was not expected."
	err := UnexpectedSlice{target: "int"}

	assert.ErrorContains(t, err, expected)
}

func Test_UnexpectedMap_Error(t *testing.T) {
	err := ErrUnexpectedMap

	assert.Error(t, err, ErrUnexpectedMap)
}
