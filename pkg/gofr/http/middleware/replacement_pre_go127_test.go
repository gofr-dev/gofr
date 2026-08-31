//go:build !go1.27

package middleware

// replacementInJSON is how encoding/json renders U+FFFD — the character it
// substitutes for invalid UTF-8 — inside a JSON string.
//
// Up to Go 1.26 that is the six-character escape \ufffd. Go 1.27 rebuilt
// encoding/json on encoding/json/v2 and writes the rune itself instead. Both
// decode to the same string; only the bytes on the wire differ, and these tests
// exist to pin those bytes, so the expectation follows the toolchain.
//
// The switch belongs to the toolchain, not to us: a Go 1.27 build emits the raw
// rune whatever go directive go.mod declares.
const replacementInJSON = `\ufffd`
