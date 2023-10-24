package types

import (
	"strconv"
	"strings"

	"gofr.dev/pkg/errors"

	"golang.org/x/text/currency"
)

// Currency represents a string type for currency values in the format "ISO 4217 Currency Code Amount".
// Example: "USD 34.55".
//
// The Check method is used to validate the Currency value. It ensures that the Currency value
// adheres to the required format and that the currency code is a valid ISO 4217 currency code.
// If the value is not valid, it returns an error specifying the invalid parameter.
type Currency string

// Check validates the Currency value by ensuring it adheres to the expected format
// and checks the validity of the ISO 4217 currency code and the currency amount.
//
// If the Currency value is not in the expected format, or the currency code or amount
// is invalid, this function returns an error indicating the specific parameter that is invalid.
// - "currency" is returned if the format is incorrect.
// - "currencyCountryCode" is returned if the ISO 4217 currency code is invalid.
// - "currencyValue" is returned if the currency amount is not a valid floating-point number.
func (c Currency) Check() error {
	const size = 64
	// Currencies MUST use the ISO 4217 currency codes. Ex: USD 34.55
	currencyArray := strings.Fields(string(c))

	const currencyArrayLen = 2
	if len(currencyArray) != currencyArrayLen {
		return errors.InvalidParam{Param: []string{"currency"}}
	}

	_, err := currency.ParseISO(currencyArray[0])
	if err != nil {
		return errors.InvalidParam{Param: []string{"currencyCountryCode"}}
	}

	_, err = strconv.ParseFloat(currencyArray[1], size)
	if err != nil {
		return errors.InvalidParam{Param: []string{"currencyValue"}}
	}

	return nil
}
