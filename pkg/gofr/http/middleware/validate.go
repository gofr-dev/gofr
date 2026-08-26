package middleware

import "strings"

func isWellKnown(path string) bool {
	return path == "/.well-known" || strings.HasPrefix(path, "/.well-known/")
}
