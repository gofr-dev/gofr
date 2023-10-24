package types

import (
	"reflect"
	"testing"

	"gofr.dev/pkg/errors"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		rules   []Rule
		wantErr error
	}{
		{"valid phone", []Rule{Phone("+17777777777")}, nil},
		{"invalid phone", []Rule{Phone("+17777777777qq")}, errors.InvalidParam{Param: []string{"Phone Number contains Non Numeric characters"}}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.rules...)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("%v Validate() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
