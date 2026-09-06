//go:build gofr_nosqldrivers

// No driver registration in a build made with -tags gofr_nosqldrivers.
//
// DB_DIALECT=postgres or =sqlite then fails at startup: NewSQL's registerOtel
// call reports database/sql's own "unknown driver" error naming the dialect, and
// returns a nil DB. That is an already-supported state -- it is what an
// unconfigured database produces, and the container guards it with isNil -- so
// the service still starts, with one error line saying exactly what is missing.
// A loud, immediate complaint is the point of doing this with a tag rather than
// by asking users to blank-import drivers themselves, where the same mistake is
// silent. A user who wants one of these dialects in a tagged build imports
// the driver in their own main package, exactly as with database/sql directly.

package sql
