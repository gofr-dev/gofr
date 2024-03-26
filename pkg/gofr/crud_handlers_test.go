package gofr

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_scanEntity(t *testing.T) {
	var invalidResource int

	type user struct {
		Id   int
		Name string
	}

	tests := []struct {
		desc  string
		input interface{}
		resp  *entity
		err   error
	}{
		{"success case", &user{}, &entity{name: "user", entityType: reflect.TypeOf(user{}), primaryKey: "id"}, nil},
		{"invalid resource", &invalidResource, nil, errors.New("unexpected resource given for CRUDFromStruct")},
	}

	for i, tc := range tests {
		resp, err := scanEntity(tc.input)

		assert.Equal(t, tc.resp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
