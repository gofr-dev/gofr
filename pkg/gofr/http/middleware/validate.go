package middleware

import "strings"

func isWellKnown(path string) bool {
	return strings.HasPrefix(path, "/.well-known")
}
