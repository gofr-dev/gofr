package terminal

import (
	"bytes"
	"strings"
	"testing"
)

func tempOutput(t *testing.T) *Output {
	t.Helper()

	var b bytes.Buffer

	return &Output{out: &b}
}

func validate(t *testing.T, o *Output, exp string) {
	t.Helper()
	out := o.out.(*bytes.Buffer)
	b := out.Bytes()

	if string(b) != exp {
		b = bytes.ReplaceAll(b, []byte("\x1b"), []byte("\\x1b"))
		exp = strings.ReplaceAll(exp, "\x1b", "\\x1b")
		t.Errorf("output does not match, expected %s, got %s", exp, string(b))
	}
}

func TestReset(t *testing.T) {
	o := tempOutput(t)
	o.Reset()

	validate(t, o, "\x1b[0m")
}

func TestRestoreScreen(t *testing.T) {
	o := tempOutput(t)
	o.RestoreScreen()

	validate(t, o, "\x1b[?47l")
}

func TestSaveScreen(t *testing.T) {
	o := tempOutput(t)
	o.SaveScreen()

	validate(t, o, "\x1b[?47h")
}

func TestAltScreen(t *testing.T) {
	o := tempOutput(t)
	o.AltScreen()

	validate(t, o, "\x1b[?1049h")
}

func TestExitAltScreen(t *testing.T) {
	o := tempOutput(t)
	o.ExitAltScreen()

	validate(t, o, "\x1b[?1049l")
}

func TestClearScreen(t *testing.T) {
	o := tempOutput(t)
	o.ClearScreen()

	validate(t, o, "\x1b[2J\x1b[1;1H")
}

func TestMoveCursor(t *testing.T) {
	o := tempOutput(t)
	o.MoveCursor(2, 2)

	validate(t, o, "\x1b[2;2H")
}

func TestHideCursor(t *testing.T) {
	o := tempOutput(t)
	o.HideCursor()

	validate(t, o, "\x1b[?25l")
}

func TestShowCursor(t *testing.T) {
	o := tempOutput(t)
	o.ShowCursor()

	validate(t, o, "\x1b[?25h")
}

func TestSaveCursorPosition(t *testing.T) {
	o := tempOutput(t)
	o.SaveCursorPosition()

	validate(t, o, "\x1b[s")
}

func TestRestoreCursorPosition(t *testing.T) {
	o := tempOutput(t)
	o.RestoreCursorPosition()

	validate(t, o, "\x1b[u")
}

func TestCursorUp(t *testing.T) {
	o := tempOutput(t)
	o.CursorUp(2)

	validate(t, o, "\x1b[2A")
}

func TestCursorDown(t *testing.T) {
	o := tempOutput(t)
	o.CursorDown(2)

	validate(t, o, "\x1b[2B")
}

func TestCursorForward(t *testing.T) {
	o := tempOutput(t)
	o.CursorForward(2)

	validate(t, o, "\x1b[2C")
}

func TestCursorBack(t *testing.T) {
	o := tempOutput(t)
	o.CursorBack(2)

	validate(t, o, "\x1b[2D")
}

func TestCursorNextLine(t *testing.T) {
	o := tempOutput(t)
	o.CursorNextLine(2)

	validate(t, o, "\x1b[2E")
}

func TestCursorPrevLine(t *testing.T) {
	o := tempOutput(t)
	o.CursorPrevLine(2)

	validate(t, o, "\x1b[2F")
}

func TestClearLine(t *testing.T) {
	o := tempOutput(t)
	o.ClearLine()

	validate(t, o, "\x1b[2K")
}

func TestClearLineLeft(t *testing.T) {
	o := tempOutput(t)
	o.ClearLineLeft()

	validate(t, o, "\x1b[1K")
}

func TestClearLineRight(t *testing.T) {
	o := tempOutput(t)
	o.ClearLineRight()

	validate(t, o, "\x1b[0K")
}

func TestClearLines(t *testing.T) {
	o := tempOutput(t)
	o.ClearLines(2)

	validate(t, o, "\x1b[2K\x1b[1A\x1b[2K\x1b[1A\x1b[2K")
}

func TestChangeScrollingRegion(t *testing.T) {
	o := tempOutput(t)
	o.ChangeScrollingRegion(2, 1)

	validate(t, o, "\x1b[2;1r")
}

func TestInsertLines(t *testing.T) {
	o := tempOutput(t)
	o.InsertLines(2)

	validate(t, o, "\x1b[2L")
}

func TestDeleteLines(t *testing.T) {
	o := tempOutput(t)
	o.DeleteLines(2)

	validate(t, o, "\x1b[2M")
}

func TestSetWindowTitle(t *testing.T) {
	o := tempOutput(t)
	o.SetWindowTitle("test title")

	validate(t, o, "\x1b]2;test title")
}
