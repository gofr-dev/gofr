package scylladb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
