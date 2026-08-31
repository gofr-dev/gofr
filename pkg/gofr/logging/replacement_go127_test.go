//go:build go1.27

package logging

// replacementInJSON is how encoding/json renders U+FFFD inside a JSON string.
// See the !go1.27 file for why this moves with the toolchain.
const replacementInJSON = "�"
