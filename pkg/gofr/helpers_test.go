package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_StringSplitEnv_EmptyString(t *testing.T) {
	res := SplitEnv("", ",")

	assert.Equal(t, 0, len(res), "Test Failed at SplitEnv for Empty String")
}

func Test_StringSplitEnv_NonEmptyString(t *testing.T) {
	res := SplitEnv("test1,test2", ",")

	assert.EqualValues(t, []string{"test1", "test2"}, res, "Test Failed at SplitEnv for Non Empty String")
}
