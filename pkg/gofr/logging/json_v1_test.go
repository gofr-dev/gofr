//go:build !goexperiment.jsonv2

package logging

// invalidUTF8Replacement is how the original encoding/json engine, used up to
// Go 1.26, writes the U+FFFD that replaces an invalid UTF-8 byte: escaped, as
// \ufffd. json_v2_test.go holds the json/v2 form.
const invalidUTF8Replacement = `\ufffd`
