package cassandra

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DestinationIsNotPointer_Error(t *testing.T) {
	err := ErrDestinationIsNotPointer

	require.Equal(t, err, ErrDestinationIsNotPointer)
}

func Test_UnexpectedPointer_Error(t *testing.T) {
	expected := "a pointer to int was not expected."
	err := ErrUnexpectedPointer{target: "int"}

	require.ErrorContains(t, err, expected)
}

func Test_UnexpectedSlice_Error(t *testing.T) {
	expected := "a slice of int was not expected."
	err := ErrUnexpectedSlice{target: "int"}

	require.ErrorContains(t, err, expected)
}

func Test_UnexpectedMap_Error(t *testing.T) {
	err := ErrUnexpectedMap

	require.ErrorIs(t, err, ErrUnexpectedMap)
}
