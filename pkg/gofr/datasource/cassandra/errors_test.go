package cassandra

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DestinationIsNotPointer_Error(t *testing.T) {
	err := errDestinationIsNotPointer

	require.Equal(t, err, errDestinationIsNotPointer)
}

func Test_UnexpectedPointer_Error(t *testing.T) {
	expected := "a pointer to int was not expected."
	err := errUnexpectedPointer{target: "int"}

	require.ErrorContains(t, err, expected)
}

func Test_UnexpectedSlice_Error(t *testing.T) {
	expected := "a slice of int was not expected."
	err := errUnexpectedSlice{target: "int"}

	require.ErrorContains(t, err, expected)
}

func Test_UnexpectedMap_Error(t *testing.T) {
	err := errUnexpectedMap

	require.ErrorIs(t, err, errUnexpectedMap)
}
