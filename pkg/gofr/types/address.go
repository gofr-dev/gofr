// Package types provides most common structures that can be used in a http server
// This package also implements a validation function for all the types that check the validity of
// the values in the fields of these structures
package types

import (
	"regexp"
	"strconv"

	"gofr.dev/pkg/errors"
)

// Address represents a structured address with various fields, such as address lines, city or town,
// state or province, postal code, and more. It is designed to store information related to a physical address.
//
// The Check method is used to validate the fields of the Address struct to ensure that they adhere to specific criteria.
// This method performs checks on mandatory fields and validates optional fields like phone numbers, email addresses,
// postal codes, and more. It returns an error if any of the validation checks fail.
type Address struct {
	AddressLines  []string `json:"addressLines"`            // Address lines.
	CityTown      string   `json:"cityTown"`                // City or town.
	StateProvince string   `json:"stateProvince"`           // State or province.
	CountryCode   string   `json:"countryCode"`             // ISO country code.
	PostalCode    string   `json:"postalCode"`              // Postal code.
	Company       string   `json:"company,omitempty"`       // Company name (optional).
	Name          string   `json:"name,omitempty"`          // Name (optional).
	Residential   bool     `json:"residential,omitempty"`   // Indicates if it is a residential address (optional).
	DeliveryPoint string   `json:"deliveryPoint,omitempty"` // Delivery point (optional).
	CarrierRoute  string   `json:"carrierRoute,omitempty"`  // Carrier route (optional).
	TaxID         string   `json:"taxId,omitempty"`         // Tax ID (optional).
	Status        string   `json:"status,omitempty"`        // Status (optional).
	Phone         Phone    `json:"phone,omitempty"`         // Phone number (optional).
	Email         Email    `json:"email,omitempty"`         // Email address (optional).
	Fax           Phone    `json:"fax,omitempty"`           // Fax number (optional).
	County        string   `json:"county,omitempty"`        // County (optional).
}

// Check performs validation checks on the Address struct fields.
//
// It verifies the format and content of the address fields to ensure they meet the required criteria.
// Specifically, it checks for mandatory fields, validates the length of address lines, verifies the
// country code and postal code, and performs additional checks on optional fields like phone numbers,
// email addresses, and more.
//
// If any of the validation checks fail, Check returns an error describing the specific parameter that is invalid.
func (addr *Address) Check() error {
	err := addr.checkMandatoryFields()
	if err != nil {
		return err
	}

	const addrLines = 3
	if len(addr.AddressLines) > addrLines {
		return errors.InvalidParam{Param: []string{"addressLines"}}
	}

	err = checkCountryCode(addr.CountryCode)
	if err != nil {
		return err
	}

	err = checkPostalCode(addr.PostalCode)
	if err != nil {
		return err
	}

	err = checkPhone(addr.Phone)
	if err != nil {
		return err
	}

	// fax has the same validation as phone.
	if err = checkPhone(addr.Fax); err != nil {
		return errors.InvalidParam{Param: []string{"fax"}}
	}

	err = checkCarrierRoute(addr.CarrierRoute)
	if err != nil {
		return err
	}

	err = checkDelivery(addr.DeliveryPoint)
	if err != nil {
		return err
	}

	err = checkEmail(addr.Email)
	if err != nil {
		return err
	}

	return nil
}

func (addr *Address) checkMandatoryFields() error {
	if len(addr.AddressLines) == 0 {
		return errors.MissingParam{Param: []string{"addressLines"}}
	}

	if addr.CityTown == "" {
		return errors.MissingParam{Param: []string{"cityTown"}}
	}

	if addr.StateProvince == "" {
		return errors.MissingParam{Param: []string{"state"}}
	}

	if addr.CountryCode == "" {
		return errors.MissingParam{Param: []string{"countryCode"}}
	}

	if addr.PostalCode == "" {
		return errors.MissingParam{Param: []string{"postalCode"}}
	}

	return nil
}

// ref: https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2
//
//nolint:gochecknoglobals // the following variable have to be defined as global
var validCountryCode = map[string]bool{"AF": true, "AX": true, "AL": true, "DZ": true, "AS": true, "AD": true, "AO": true, "AI": true,
	"AQ": true, "AG": true, "AR": true, "AM": true, "AW": true, "AU": true, "AT": true, "AZ": true, "BS": true, "BH": true, "BD": true,
	"BB": true, "BY": true, "BE": true, "BZ": true, "BJ": true, "BM": true, "BT": true, "BO": true, "BQ": true, "BA": true, "BW": true,
	"BV": true, "BR": true, "IO": true, "BN": true, "BG": true, "BF": true, "BI": true, "CV": true, "KH": true, "CM": true, "CA": true,
	"KY": true, "CF": true, "TD": true, "CL": true, "CN": true, "CX": true, "CC": true, "CO": true, "KM": true, "CG": true, "CD": true,
	"CK": true, "CR": true, "CI": true, "HR": true, "CU": true, "CW": true, "CY": true, "CZ": true, "DK": true, "DJ": true, "DM": true,
	"DO": true, "EC": true, "EG": true, "SV": true, "GQ": true, "ER": true, "EE": true, "SZ": true, "ET": true, "FK": true, "FO": true,
	"FJ": true, "FI": true, "FR": true, "GF": true, "PF": true, "TF": true, "GA": true, "GM": true, "GE": true, "DE": true, "GH": true,
	"GI": true, "GR": true, "GL": true, "GD": true, "GP": true, "GU": true, "GT": true, "GG": true, "GN": true, "GW": true, "GY": true,
	"HT": true, "HM": true, "VA": true, "HN": true, "HK": true, "HU": true, "IS": true, "IN": true, "ID": true, "IR": true, "IQ": true,
	"IE": true, "IM": true, "IL": true, "IT": true, "JM": true, "JP": true, "JE": true, "JO": true, "KZ": true, "KE": true, "KI": true,
	"KP": true, "KR": true, "KW": true, "KG": true, "LA": true, "LV": true, "LB": true, "LS": true, "LR": true, "LY": true, "LI": true,
	"LT": true, "LU": true, "MO": true, "MG": true, "MW": true, "MY": true, "MV": true, "ML": true, "MT": true, "MH": true, "MQ": true,
	"MR": true, "MU": true, "YT": true, "MX": true, "FM": true, "MD": true, "MC": true, "MN": true, "ME": true, "MS": true, "MA": true,
	"MZ": true, "MM": true, "NA": true, "NR": true, "NP": true, "NL": true, "NC": true, "NZ": true, "NI": true, "NE": true, "NG": true,
	"NU": true, "NF": true, "MK": true, "MP": true, "NO": true, "OM": true, "PK": true, "PW": true, "PS": true, "PA": true, "PG": true,
	"PY": true, "PE": true, "PH": true, "PN": true, "PL": true, "PT": true, "PR": true, "QA": true, "RE": true, "RO": true, "RU": true,
	"RW": true, "BL": true, "SH": true, "KN": true, "LC": true, "MF": true, "PM": true, "VC": true, "WS": true, "SM": true, "ST": true,
	"SA": true, "SN": true, "RS": true, "SC": true, "SL": true, "SG": true, "SX": true, "SK": true, "SI": true, "SB": true, "SO": true,
	"ZA": true, "GS": true, "SS": true, "ES": true, "LK": true, "SD": true, "SR": true, "SJ": true, "SE": true, "CH": true, "SY": true,
	"TW": true, "TJ": true, "TZ": true, "TH": true, "TL": true, "TG": true, "TK": true, "TO": true, "TT": true, "TN": true, "TR": true,
	"TM": true, "TC": true, "TV": true, "UG": true, "UA": true, "AE": true, "GB": true, "UM": true, "US": true, "UY": true, "UZ": true,
	"VU": true, "VE": true, "VN": true, "VG": true, "VI": true, "WF": true, "EH": true, "YE": true, "ZM": true, "ZW": true}

func checkCountryCode(countrycode string) error {
	if !validCountryCode[countrycode] {
		return errors.InvalidParam{Param: []string{"countryCode"}}
	}

	return nil
}

// this will compile the regex once instead of compiling it each time when it is being called.
var postRegex = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9\- ]{0,10}[a-z0-9]$`)

func checkPostalCode(postal string) error {
	if !postRegex.MatchString(postal) {
		return errors.InvalidParam{Param: []string{"postalCode"}}
	}

	return nil
}

func checkDelivery(deliveryPoint string) error {
	// ref:https://en.wikipedia.org/wiki/Delivery_point
	if deliveryPoint != "" {
		const deliveryPointLen = 2
		if len(deliveryPoint) != deliveryPointLen {
			return errors.InvalidParam{Param: []string{"deliveryPoint"}}
		}

		dp, err := strconv.Atoi(deliveryPoint)
		if err != nil || (dp < 0 || dp > 99) {
			return errors.InvalidParam{Param: []string{"deliveryPoint"}}
		}
	}

	return nil
}

func checkCarrierRoute(carrierRoute string) error {
	if carrierRoute != "" {
		const carrierRouteLen = 4
		if len(carrierRoute) != carrierRouteLen {
			return errors.InvalidParam{Param: []string{"carrierRoute"}}
		}

		_, err := strconv.Atoi(carrierRoute)
		if err != nil {
			return errors.InvalidParam{Param: []string{"carrierRoute"}}
		}
	}

	return nil
}

func checkPhone(phone Phone) error {
	if phone != "" {
		err := Validate(phone)
		if err != nil {
			return errors.InvalidParam{Param: []string{"phoneNumber"}}
		}
	}

	return nil
}

func checkEmail(email Email) error {
	if email != "" {
		err := Validate(email)
		if err != nil {
			return errors.InvalidParam{Param: []string{"emailAddress"}}
		}
	}

	return nil
}
