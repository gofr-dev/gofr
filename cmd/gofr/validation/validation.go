package validation

import (
	"fmt"
	"sort"
	"strings"

	"gofr.dev/pkg/errors"
)

// ValidateParams validates parameters against a list of valid parameters and checks for the presence of mandatory parameters.
func ValidateParams(params map[string]string, validParams map[string]bool, mandatoryParams *[]string) error {
	var (
		invalidParams          []string
		missingMandatoryParams []string
		i                      = 0
	)

	keys := make([]string, len(params))

	for key := range params {
		keys[i] = key
		i++
	}

	// sort params to retain consistent orders while checking
	sort.Strings(keys)

	// invalid parameters check
	for _, key := range keys {
		if !validParams[key] {
			invalidParams = append(invalidParams, key)
		}
	}

	if len(invalidParams) > 0 {
		errorString := fmt.Sprintf(`unknown parameter(s) [` + strings.Join(invalidParams, ",") + `]. ` +
			`Run gofr <command_name> -h for help of the command.`)

		return &errors.Response{Reason: errorString}
	}

	// mandatory parameters check
	for _, key := range *mandatoryParams {
		if params[key] == "" {
			missingMandatoryParams = append(missingMandatoryParams, key)
		}
	}

	if len(missingMandatoryParams) > 0 {
		return errors.MissingParam{Param: missingMandatoryParams}
	}

	return nil
}
