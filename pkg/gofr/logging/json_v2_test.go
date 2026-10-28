//go:build goexperiment.jsonv2

package logging

// invalidUTF8Replacement is how json/v2, the engine encoding/json runs on by
// default from Go 1.27, writes the U+FFFD that replaces an invalid UTF-8 byte:
// as the raw character. Both forms decode to the same string. json_v1_test.go
// holds the original engine's form.
const invalidUTF8Replacement = "�"
