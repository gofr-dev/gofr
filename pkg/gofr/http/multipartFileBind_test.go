package http

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getFileName(t *testing.T) {
	testStruct := struct {
		A string `file:"A"`
		B string
		c string
	}{
		A: "A",
		B: "B",
		c: "c",
	}

	val := reflect.ValueOf(testStruct)

	// Field A
	f1 := val.Type().Field(0)
	a, ok := getFileName(&f1)
	assert.Equal(t, "A", a)
	assert.True(t, ok)

	// Field B
	f2 := val.Type().Field(1)
	b, ok := getFileName(&f2)
	assert.Equal(t, "B", b)
	assert.True(t, ok)

	// Field C
	f3 := val.Type().Field(2)
	c, ok := getFileName(&f3)
	assert.Equal(t, "", c)
	assert.False(t, ok)
}
