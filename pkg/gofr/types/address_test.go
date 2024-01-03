package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestAddress_Check(t *testing.T) {
	tests := []struct {
		name      string
		addstruct Address
		err       error
	}{
		{"addressLine value missing", Address{AddressLines: []string{}}, errors.MissingParam{Param: []string{"addressLines"}}},
		{"cityTown value missing", Address{AddressLines: []string{"banglored"}}, errors.MissingParam{Param: []string{"cityTown"}}},
		{"state value missing", Address{AddressLines: []string{"banglored"},
			CityTown: "bengaluru"}, errors.MissingParam{Param: []string{"state"}}},
		{"countryCode value missing", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka"}, errors.MissingParam{Param: []string{"countryCode"}}},
		{"checkCountryCode value missing", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "AA", PostalCode: "560043"}, errors.InvalidParam{Param: []string{"countryCode"}}},
		{"checkPostalCode value missing", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IN"}, errors.MissingParam{Param: []string{"postalCode"}}},
		{"checkPostalCode value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IN", PostalCode: "56@tree"}, errors.InvalidParam{Param: []string{"postalCode"}}},
		{"correct address", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "US", PostalCode: "560043"}, nil},
		{"phone number value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "JO", PostalCode: "560043", Phone: "erewrwe"},
			errors.InvalidParam{Param: []string{"phoneNumber"}}},
		{"correct address with phone", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IS", PostalCode: "560043", Phone: "+1234567890"}, nil},
		{"fax value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "LU", PostalCode: "560043", Fax: "erewrwe"}, errors.InvalidParam{Param: []string{"fax"}}},
		{"correct address with fax", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IN", PostalCode: "560043", Fax: "+1234567890"}, nil},
		{"email value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru", StateProvince: "karnataka",
			CountryCode: "IN", PostalCode: "560043", Email: "+1234567890@name*.com"}, errors.InvalidParam{Param: []string{"emailAddress"}}},
		{"correct address with email", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IN", PostalCode: "560043", Email: "nameingit@gmail.com"}, nil},
		{"carrier route incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru", StateProvince: "karnataka",
			CountryCode: "IN", PostalCode: "560043", CarrierRoute: "aaaa"}, errors.InvalidParam{Param: []string{"carrierRoute"}}},
		{"carrier route incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru", StateProvince: "karnataka",
			CountryCode: "IN", PostalCode: "560043", CarrierRoute: "aaa"}, errors.InvalidParam{Param: []string{"carrierRoute"}}},
		{"correct address with carrier", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IN", PostalCode: "560043", CarrierRoute: "2323"}, nil},
		{"delivery value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru", StateProvince: "karnataka",
			CountryCode: "IN", PostalCode: "560043", DeliveryPoint: "a"}, errors.InvalidParam{Param: []string{"deliveryPoint"}}},
		{"delivery value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru", StateProvince: "karnataka",
			CountryCode: "IN", PostalCode: "560043", DeliveryPoint: "aa"}, errors.InvalidParam{Param: []string{"deliveryPoint"}}},
		{"delivery value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru", StateProvince: "karnataka",
			CountryCode: "IN", PostalCode: "560043", DeliveryPoint: "-19"}, errors.InvalidParam{Param: []string{"deliveryPoint"}}},
		{"delivery value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru", StateProvince: "karnataka",
			CountryCode: "IN", PostalCode: "560043", DeliveryPoint: "102"}, errors.InvalidParam{Param: []string{"deliveryPoint"}}},
		{"correct address with delivery", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IN", PostalCode: "560043", DeliveryPoint: "23"}, nil},
		{"correct address with delivery", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IN", PostalCode: "560043", DeliveryPoint: "23"}, nil},
		{"incorrect address with delivery", Address{AddressLines: []string{"banglored", "d", "w", "d"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "AL", PostalCode: "560043", DeliveryPoint: "23"},
			errors.InvalidParam{Param: []string{"addressLines"}}},
		{"country code value incorrect", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru", StateProvince: "karnataka",
			CountryCode: "I", PostalCode: "560043", DeliveryPoint: "23"}, errors.InvalidParam{Param: []string{"countryCode"}}},
		{"address with county", Address{AddressLines: []string{"banglored"}, CityTown: "bengaluru",
			StateProvince: "karnataka", CountryCode: "IN", PostalCode: "560043", DeliveryPoint: "23", County: "Franklin"}, nil},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.addstruct)

			assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n%s", i, tt.name)
		})
	}
}

func TestCountyOmitEmpty(t *testing.T) {
	a := Address{County: ""}
	b, _ := json.Marshal(a)

	if strings.Contains(string(b), "county") {
		t.Errorf("The key `county` should not be present")
	}
}
