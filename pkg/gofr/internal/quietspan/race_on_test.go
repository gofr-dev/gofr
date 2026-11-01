//go:build race

package quietspan

// The race detector allocates on its own schedule, so allocation counts are not checked under it.
const raceEnabled = true
