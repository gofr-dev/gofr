//go:build !gofr_nosqldrivers

// Driver registration for the dialects GoFr resolves by name.
//
// These are blank imports whose only effect is each driver's init() calling
// database/sql.Register, so that sql.Open("sqlite", ...) can find it. They are
// behind a build tag because they are expensive out of proportion to how often
// they are used: modernc.org/sqlite pulls modernc.org/libc, whose netdb init
// parses embedded copies of /etc/protocols and /etc/services into permanent Go
// structs -- 1.69 MB of retained heap, about half of GoFr's fixed heap floor,
// in every binary whether or not SQLite is opened.
//
// The default build includes this file, so nothing changes unless a user asks
// for it with -tags gofr_nosqldrivers. The MySQL driver is NOT here: sql.go
// imports it non-blank for its config types, so it is linked either way.

package sql

import (
	_ "github.com/lib/pq"  // registers the "postgres" dialect.
	_ "modernc.org/sqlite" // registers the "sqlite" dialect.
)
