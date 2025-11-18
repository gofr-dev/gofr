package service

import "fmt"

// ValidationMode determines how validation failures are handled.
type ValidationMode int

const (
	// ValidationModeSoft allows the service to continue with logged warnings when validation fails.
	// This is the default mode for backward compatibility.
	ValidationModeSoft ValidationMode = iota

	// ValidationModeStrict causes service creation to fail immediately when validation fails.
	// Recommended for production environments to ensure proper configuration.
	ValidationModeStrict
)

// Validator defines an interface for validating HTTP service options.
// Options that implement this interface will have their Validate method called during service initialization.
type Validator interface {
	// Validate checks if the option configuration is valid.
	// Returns an error if validation fails, nil otherwise.
	Validate() error

	// FeatureName returns the name of the feature for logging purposes.
	FeatureName() string
}

// ValidationConfig holds configuration for service option validation.
type ValidationConfig struct {
	// Mode determines how validation failures are handled.
	// Default is ValidationModeSoft for backward compatibility.
	Mode ValidationMode

	// Logger is used to log validation warnings/errors.
	// If nil, validation errors will only be returned in strict mode.
	Logger Logger
}

// validateOptions validates all options that implement the Validator interface.
// Returns an error in strict mode if any validation fails, otherwise logs warnings and continues.
func validateOptions(config ValidationConfig, options []Options) error {
	var validationErrors []ValidationError

	for _, opt := range options {
		if validator, ok := opt.(Validator); ok {
			if err := validator.Validate(); err != nil {
				validationErr := ValidationError{
					Feature: validator.FeatureName(),
					Err:     err,
				}
				validationErrors = append(validationErrors, validationErr)

				// Log warning in soft mode or if logger is available
				if config.Mode == ValidationModeSoft && config.Logger != nil {
					config.Logger.Log(fmt.Sprintf(
						"Warning: Validation failed for %s: %v. Service will continue with potentially incorrect configuration.",
						validationErr.Feature, err))
				}
			}
		}
	}

	// In strict mode, return error if any validation failed
	if config.Mode == ValidationModeStrict && len(validationErrors) > 0 {
		return &ValidationErrors{
			Errors: validationErrors,
		}
	}

	return nil
}

// ValidationError represents a single validation failure for a feature.
type ValidationError struct {
	Feature string
	Err     error
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %v", e.Feature, e.Err)
}

// ValidationErrors represents multiple validation failures.
type ValidationErrors struct {
	Errors []ValidationError
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}

	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	msg := fmt.Sprintf("%d validation errors: ", len(e.Errors))
	for i, err := range e.Errors {
		if i > 0 {
			msg += "; "
		}
		msg += err.Error()
	}

	return msg
}

