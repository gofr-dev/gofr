package types

import (
	"time"

	"gofr.dev/pkg/errors"
)

// Datetime represents a date and time along with timezone information in RFC3339 format.
//
// Datetime values consist of two fields:
// - Value: A string field that holds the full date and time in RFC3339 format.
// - Timezone: A string field that holds the timezone information about the Datetime value.
//
// This type provides functionality for validating Datetime values, including format and timezone.
type Datetime struct {
	Value    string `json:"value"`
	Timezone string `json:"timezone"`
}

// Check validates the Datetime value.
func (d Datetime) Check() error {
	// date and time together MUST always be included in the datetime structure
	_, err := time.Parse(time.RFC3339, d.Value)
	if err != nil {
		return errors.InvalidParam{Param: []string{"datetime"}}
	}

	err = Validate(TimeZone(d.Timezone))
	if err != nil {
		return errors.InvalidParam{Param: []string{"timezone"}}
	}

	return nil
}
