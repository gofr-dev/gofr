package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetColor(t *testing.T) {
	tests := []struct {
		name      string
		colorCode int
		expected  string
	}{
		{"black", Black, "\x1b[38;5;0m"},
		{"red", Red, "\x1b[38;5;1m"},
		{"green", Green, "\x1b[38;5;2m"},
		{"white", White, "\x1b[38;5;7m"},
		{"bright white", BrightWhite, "\x1b[38;5;15m"},
		// SetColor does not validate its argument: any 256-color palette index is written
		// through as-is, which is what callers reaching past the named constants rely on.
		{"unnamed 256-color index", 208, "\x1b[38;5;208m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := tempOutput(t)
			o.SetColor(tc.colorCode)

			validate(t, o, tc.expected)
		})
	}
}

func TestResetColor(t *testing.T) {
	o := tempOutput(t)
	o.ResetColor()

	validate(t, o, "\x1b[0m")
}

// TestSetColorResetColor covers the pairing as it is actually used -- color on, write, color
// off -- which is the only place ResetColor's job of undoing SetColor is observable.
func TestSetColorResetColor(t *testing.T) {
	o := tempOutput(t)

	o.SetColor(Green)
	o.Print("gofr")
	o.ResetColor()

	validate(t, o, "\x1b[38;5;2mgofr\x1b[0m")
}

// TestColorConstants pins the color codes to their ANSI 256-color palette indices. They are
// written straight into the escape sequence, so inserting or reordering a constant repaints
// every caller's output with no other signal that anything changed.
func TestColorConstants(t *testing.T) {
	got := []int{
		Black, Red, Green, Yellow, Blue, Magenta, Cyan, White,
		BrightBlack, BrightRed, BrightGreen, BrightYellow,
		BrightBlue, BrightMagenta, BrightCyan, BrightWhite,
	}

	want := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

	assert.Equal(t, want, got, "color constants must keep their ANSI 256-color palette indices")
}
