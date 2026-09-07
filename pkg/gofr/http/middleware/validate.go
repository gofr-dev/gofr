package middleware

import "strings"

const wellKnownPrefix = "/.well-known"

func isWellKnown(path string) bool {
	return path == wellKnownPrefix || strings.HasPrefix(path, wellKnownPrefix+"/")
}
