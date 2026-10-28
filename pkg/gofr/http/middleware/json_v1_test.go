//go:build !goexperiment.jsonv2

package middleware

// Values pinned for the original encoding/json engine, used up to Go 1.26 (and on
// later Go with GOEXPERIMENT=nojsonv2). json_v2_test.go holds the same names for
// the json/v2 engine that encoding/json runs on by default from Go 1.27.

// invalidUTF8Replacement is how encoding/json writes the U+FFFD that replaces an
// invalid UTF-8 byte: escaped, as \ufffd.
const invalidUTF8Replacement = `\ufffd`

// requestLogAllocs is the exact allocation count of one request through the
// Logging middleware; see TestLoggingChainAllocationsDoNotRegress.
const requestLogAllocs = 8
