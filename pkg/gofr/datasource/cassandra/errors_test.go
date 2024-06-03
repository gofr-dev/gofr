package cassandra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DestinationIsNotPointer_Error(t *testing.T) {
	expected := "destination is not pointer"
	err := DestinationIsNotPointer{}
	result := err.Error()

	assert.Equal(t, expected, result, "Error message should be 'destination is not pointer'")
}

func Test_UnexpectedPointer_Error(t *testing.T) {
	expected := "a pointer to int was not expected."
	err := UnexpectedPointer{target: "int"}
	result := err.Error()

	assert.Equal(t, expected, result, "Error message should be 'destination is not pointer'")
}
