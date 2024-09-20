package gofr

import "regexp"

// Precompiled regex patterns to be used throughout the package
var (
	// Match one or more whitespace characters
	MatchSpaces = regexp.MustCompile(`\s+`)

	// Match patterns like */2, 1-59/5
	MatchN = regexp.MustCompile(`(.*)/(\d+)`)

	// Match ranges like 1-5, 10-15
	MatchRange = regexp.MustCompile(`^(\d+)-(\d+)$`)
)
