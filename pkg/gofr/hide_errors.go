package gofr

import (
	"strings"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
)

const hideInternalErrorsKey = "HIDE_INTERNAL_ERRORS"

// readHideInternalErrors reports whether HIDE_INTERNAL_ERRORS is set to true. Unset or false keeps
// error messages as they are; any other value is logged once and treated as false, so a typo never
// changes what clients receive.
func readHideInternalErrors(cfg config.Config, logger logging.Logger) bool {
	value := strings.TrimSpace(cfg.Get(hideInternalErrorsKey))

	switch {
	case value == "" || strings.EqualFold(value, "false"):
		return false
	case strings.EqualFold(value, "true"):
		return true
	default:
		logger.Warnf("invalid value of config %s: %q, expected true or false; error messages are not hidden",
			hideInternalErrorsKey, value)

		return false
	}
}
