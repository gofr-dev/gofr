//go:build goexperiment.jsonv2 && !go1.27

package middleware

// Values for the experimental json/v2 engine that GOEXPERIMENT=jsonv2 builds into
// encoding/json on Go 1.25 and 1.26. It writes like the Go 1.27 engine
// (json_v2_test.go) but is a preview, not the engine any supported default ships.

// invalidUTF8Replacement is how encoding/json writes the U+FFFD that replaces an
// invalid UTF-8 byte: as the raw character, as from Go 1.27.
const invalidUTF8Replacement = "�"

// requestLogAllocs is 0: the count is not pinned here. The preview engine
// allocates 16 times per request, against 9 for the Go 1.27 engine, and pinning a
// preview's number would make the guard track the experiment rather than this
// package's optimizations. TestLoggingChainAllocationsDoNotRegress skips.
const requestLogAllocs = 0
