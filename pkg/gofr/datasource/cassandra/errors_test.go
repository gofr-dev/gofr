package cassandra

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DestinationIsNotPointer_Error(t *testing.T) {
	err := destinationIsNotPointer{}

	require.ErrorContains(t, err, msgDestinationIsNotPointer)
}

func Test_UnexpectedPointer_Error(t *testing.T) {
	expected := "a pointer to int was not expected."
	err := unexpectedPointer{target: "int"}

	require.ErrorContains(t, err, expected)
}

func Test_UnexpectedSlice_Error(t *testing.T) {
	expected := "a slice of int was not expected."
	err := unexpectedSlice{target: "int"}

	require.ErrorContains(t, err, expected)
}

func Test_UnexpectedMap_Error(t *testing.T) {
	err := unexpectedMap{}

	require.ErrorContains(t, err, msgUnexpectedMap)
}
