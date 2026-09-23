package sql

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

// TestRegisterOtel_UnregisteredDialectFailsLoudly pins the mechanism the whole
// gofr_nosqldrivers tag rests on.
//
// With the tag, nothing calls sql.Register for postgres or sqlite, so a service
// configured with DB_DIALECT=postgres has to fail visibly rather than silently.
// The thing that makes it visible is otelsql.Register: it calls sql.Open with the
// driver name (XSAM/otelsql v0.43.0, sql.go:41), which returns database/sql's
// "unknown driver" error, and NewSQL reports it and returns nil -- the same state
// an unconfigured database produces, which the container already guards with
// isNil.
//
// It is asserted against a name that is never registered in EITHER build rather
// than against postgres under the tag, and that is deliberate.
// drivers_testdeps_test.go blank-imports pq and modernc.org/sqlite so the rest of
// the suite behaves identically in both configurations -- which means the test
// binary DOES have those drivers even when the library does not, and a tagged
// assertion about postgres would be testing the fixture rather than the code. A
// dialect nothing registers exercises the same path in both builds, so it cannot
// quietly stop running.
//
// A dependency bump that changed this error, or made Register succeed for an
// unknown driver, would turn the tag's loud failure into a silent one. That is
// what this catches.
//
// The other half -- that -tags gofr_nosqldrivers really does leave postgres and
// sqlite unregistered in a user's binary -- is not observable from inside a test
// binary that imports the drivers itself. It is asserted from outside, by the
// "Each tag removes the packages it claims to" step in .github/workflows/go.yml,
// which fails if github.com/lib/pq or modernc.org/sqlite is still linked under
// the tag.
//
// This file deliberately has no build tag: both halves of the mechanism it pins
// -- the aliasing and the unknown-driver error -- are the same in either build,
// and a tagged copy would only run in one job.
func TestRegisterOtel_UnregisteredDialectFailsLoudly(t *testing.T) {
	_, err := registerOtel("gofr-no-such-driver", logging.NewMockLogger(logging.DEBUG))

	require.Error(t, err, "an unregistered dialect must not register successfully")
	require.Contains(t, strings.ToLower(err.Error()), "unknown driver",
		"the error naming the missing driver is what makes a tagged build's misconfiguration legible")
}

// TestRegisterOtel_AliasedDialectsUsePostgres pins the aliasing, which decides
// which dialects the tag actually affects.
//
// supabase and cockroachdb are registered as postgres (sql.go:268), so they need
// the postgres driver exactly as DB_DIALECT=postgres does -- a fact the tag's own
// documentation has to state, or a user reads "postgres and sqlite" and is
// surprised by a failing supabase build.
func TestRegisterOtel_AliasedDialectsUsePostgres(t *testing.T) {
	for _, dialect := range []string{supabaseDialect, cockroachDB} {
		name, err := registerOtel(dialect, logging.NewMockLogger(logging.DEBUG))

		require.NoError(t, err, "%s resolves through the postgres driver", dialect)
		require.Contains(t, name, dialectPostgres,
			"%s must register under the postgres driver, so gofr_nosqldrivers affects it too", dialect)
	}
}
