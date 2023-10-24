package types

import (
	"github.com/google/uuid"

	"gofr.dev/pkg/errors"
)

// ValidateUUID validates a slice of UUID strings and returns an error if any are invalid.
func ValidateUUID(data ...string) error {
	var parseErrors errors.InvalidParam

	for _, val := range data {
		_, err := uuid.Parse(val)
		if err != nil {
			parseErrors.Param = append(parseErrors.Param, val)
		}
	}

	if parseErrors.Param != nil {
		return parseErrors
	}

	return nil
}
