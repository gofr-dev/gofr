package cassandra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DestinationIsNotPointer_Error(t *testing.T) {
	expected := "destination is not pointer"
	err := DestinationIsNotPointer{}
	result := err.Error()

	assert.Equal(t, expected, result)
}

func Test_UnexpectedPointer_Error(t *testing.T) {
	expected := "a pointer to int was not expected."
	err := UnexpectedPointer{target: "int"}
	result := err.Error()

	assert.Equal(t, expected, result)
}

func Test_UnexpectedSlice_Error(t *testing.T) {
	expected := "a slice of int was not expected."
	err := UnexpectedSlice{target: "int"}
	result := err.Error()

	assert.Equal(t, expected, result)
}

func Test_UnexpectedMap_Error(t *testing.T) {
	expected := "a map was not expected."
	err := UnexpectedMap{}
	result := err.Error()

	assert.Equal(t, expected, result)
}
