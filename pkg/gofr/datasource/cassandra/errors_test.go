package cassandra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DestinationIsNotPointer_Error(t *testing.T) {
	err := destinationIsNotPointer{}

	assert.ErrorContains(t, err, msgDestinationIsNotPointer)
}

func Test_UnexpectedPointer_Error(t *testing.T) {
	expected := "a pointer to int was not expected."
	err := unexpectedPointer{target: "int"}

	assert.ErrorContains(t, err, expected)
}

func Test_UnexpectedSlice_Error(t *testing.T) {
	expected := "a slice of int was not expected."
	err := unexpectedSlice{target: "int"}

	assert.ErrorContains(t, err, expected)
}

func Test_UnexpectedMap_Error(t *testing.T) {
	err := unexpectedMap{}

	assert.ErrorContains(t, err, msgUnexpectedMap)
}
