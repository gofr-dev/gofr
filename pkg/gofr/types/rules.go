package types

// Rule provides the functionality to implement a validation rule
type Rule interface {
	// Check provides functionality to perform validations
	Check() error
}

// Validate takes a list of validation rules and checks them one by one.
// It returns the first error encountered, or nil if all rules pass.
func Validate(rules ...Rule) error {
	for _, rule := range rules {
		err := rule.Check()
		if err != nil {
			return err
		}
	}

	return nil
}
