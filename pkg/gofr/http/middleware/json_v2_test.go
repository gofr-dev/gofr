//go:build goexperiment.jsonv2 && go1.27

package middleware

// Values pinned for the json/v2 engine that encoding/json runs on by default from
// Go 1.27. json_v1_test.go holds the same names for the original engine, and
// json_v2_experiment_test.go for the GOEXPERIMENT=jsonv2 preview on Go 1.25 and
// 1.26.

// invalidUTF8Replacement is how encoding/json writes the U+FFFD that replaces an
// invalid UTF-8 byte: as the raw character. Both forms decode to the same string.
const invalidUTF8Replacement = "�"

// requestLogAllocs is the exact allocation count of one request through the
// Logging middleware; see TestLoggingChainAllocationsDoNotRegress. It is one
// more than with the original engine, all of it inside encoding/json: json/v2
// copies values to the heap to make them addressable (twice per entry, where the
// original engine allocated once in time.Time.MarshalJSON). The middleware's own
// allocations are unchanged.
const requestLogAllocs = 9
