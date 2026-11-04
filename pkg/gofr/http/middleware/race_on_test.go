//go:build race

package middleware

// The race detector allocates on its own schedule, so allocation counts are not checked under it.
const raceEnabled = true
