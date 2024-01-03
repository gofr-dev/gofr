package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestCurrency_Check(t *testing.T) {
	tests := []struct {
		name     string
		currency Currency
		err      error
	}{
		{"empty string passed as currency", "", errors.InvalidParam{Param: []string{"currency"}}},
		{"wrong currency code passed", "usuk 22", errors.InvalidParam{Param: []string{"currencyCountryCode"}}},
		{"wrong currency value passed", "USD ABCD", errors.InvalidParam{Param: []string{"currencyValue"}}},
		{"correct currency format", "USD 10.15", nil},
		{"currency format with negative value", "USD -66.6", nil},
		{"wrong currency format passed", "USD ABCD efg", errors.InvalidParam{Param: []string{"currency"}}},
		{"symbolic representation of currency", "$ 123.00", errors.InvalidParam{Param: []string{"currencyCountryCode"}}},
	}

	for i, tt := range tests {
		tt := tt
		err := Validate(tt.currency)

		assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n%s", i, tt.name)
	}
}
