package cassandra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DestinationIsNotPointer_Error(t *testing.T) {
	err := destinationIsNotPointer{}
	result := err.Error()

	assert.Equal(t, msgDestinationIsNotPointer, result)
}

func Test_UnexpectedPointer_Error(t *testing.T) {
	expected := "a pointer to int was not expected."
	err := unexpectedPointer{target: "int"}
	result := err.Error()

	assert.Equal(t, expected, result)
}

func Test_UnexpectedSlice_Error(t *testing.T) {
	expected := "a slice of int was not expected."
	err := unexpectedSlice{target: "int"}
	result := err.Error()

	assert.Equal(t, expected, result)
}

func Test_UnexpectedMap_Error(t *testing.T) {
	err := unexpectedMap{}
	result := err.Error()

	assert.Equal(t, msgUnexpectedMap, result)
}
