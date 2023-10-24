package gofr

import "strings"

// Filter will have the condition and the comma separated list of values in the string having the same condition.
// Check out TestContext_Filters in context_test.go for examples
type Filter map[string]string

// Filters will be a map of string to Filter struct where the field will be the key.
type Filters map[string]Filter

func (f Filters) putToFilters(key, condition, value string) {
	if _, ok := f[key]; ok {
		f[key][condition] = value
	} else {
		f[key] = Filter{condition: value}
	}
}

// DefaultFilterCondition is to be used when there is no condition is provided in the request
const DefaultFilterCondition = "eq"

// Filters parses and extracts filter parameters from the request's query parameters. It looks for parameters with a
// specific format (filter.field.condition) and constructs a Filters object containing the extracted filter conditions.
// These filters are typically used to filter query results based on specific field-value conditions.
// The method returns a Filters object, allowing further processing of filter conditions within the context of the request handling.
func (c *Context) Filters() Filters {
	allParams := c.Params()
	allFilters := Filters{}

	for param := range allParams {
		if strings.Contains(param, "filter.") {
			fields := strings.Split(param, ".")
			if len(fields) == 2 { //nolint:gomnd // This is because of fixed format.
				// If there is no condition, we need a default condition in order to have it in map
				allFilters.putToFilters(fields[1], DefaultFilterCondition, allParams[param])
			}

			if len(fields) == 3 { //nolint:gomnd // This is because of fixed format.
				allFilters.putToFilters(fields[1], fields[2], allParams[param])
			}
		}
	}

	return allFilters
}
