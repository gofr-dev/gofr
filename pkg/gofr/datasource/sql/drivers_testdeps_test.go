package sql

// The dialect drivers the tests exercise, imported for tests only.
//
// The non-test registration lives behind the gofr_nosqldrivers build tag, so
// without this the suite would pass in a default build and fail in a tagged one
// -- not because the code under test changed, but because the tests' own
// fixtures had silently lost their drivers. A test-only import keeps the suite
// identical in both configurations and cannot reach a user's binary.
import (
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)
