package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestDatetime_Check(t *testing.T) {
	tests := []struct {
		name     string
		dateTime Datetime
		err      error
	}{
		{"empty datetime struct", Datetime{}, errors.InvalidParam{Param: []string{"datetime"}}},
		{"empty timezone struct", Datetime{Value: "2018-07-14T05:00:00Z", Timezone: ".."}, errors.InvalidParam{Param: []string{"timezone"}}},
		{"correct datetime struct", Datetime{Value: "2018-07-14T05:00:00Z", Timezone: "America/New_York"}, nil},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.dateTime)

			assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n%s", i, tt.name)
		})
	}
}

func Test_DatetimeJson(t *testing.T) {
	tests := []struct {
		name      string
		addstruct interface{}
		err       error
	}{
		{"datetime value marshal fail", make(chan int), errors.InvalidParam{Param: []string{"datetime"}}},
		{"datetime value unmarshal fail ", struct {
			Value []string
		}{Value: []string{"hello"}}, errors.InvalidParam{Param: []string{"datetime"}}},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(Datetime{})

			assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n%s", i, tt.name)
		})
	}
}
