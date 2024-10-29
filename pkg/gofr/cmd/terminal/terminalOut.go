package terminal

import (
	"fmt"
	"strings"
)

const (
	// escape Escape character.
	escape = '\x1b'
	// csi Control Sequence Introducer.
	csi = string(escape) + "["
	// osc Operating System Command.
	osc = string(escape) + "]"
)

// Sequence definitions.
const (
	// Cursor positioning
	cursorUpSeq              = "%dA"
	cursorDownSeq            = "%dB"
	cursorForwardSeq         = "%dC"
	cursorBackSeq            = "%dD"
	cursorNextLineSeq        = "%dE"
	cursorPreviousLineSeq    = "%dF"
	cursorPositionSeq        = "%d;%dH"
	eraseDisplaySeq          = "%dJ"
	eraseLineSeq             = "%dK"
	saveCursorPositionSeq    = "s"
	restoreCursorPositionSeq = "u"
	changeScrollingRegionSeq = "%d;%dr"
	insertLineSeq            = "%dL"
	deleteLineSeq            = "%dM"

	// Explicit values ersasing lines
	eraseLineRightSeq  = "0K"
	eraseLineLeftSeq   = "1K"
	eraseEntireLineSeq = "2K"

	// Screen
	restoreScreenSeq  = "?47l"
	saveScreenSeq     = "?47h"
	altScreenSeq      = "?1049h"
	exitAltScreenSeq  = "?1049l"
	setWindowTitleSeq = "2;%s"
	showCursorSeq     = "?25h"
	hideCursorSeq     = "?25l"
)

const (
	moveCursorUp = iota + 1
	clearScreen
)

// Reset the terminal to its default style, removing any active styles.
func (o *Output) Reset() {
	fmt.Fprint(o.out, csi+"0"+"m")
}

// RestoreScreen restores a previously saved screen state.
func (o *Output) RestoreScreen() {
	fmt.Fprint(o.out, csi+restoreScreenSeq)
}

// SaveScreen saves the screen state.
func (o *Output) SaveScreen() {
	fmt.Fprint(o.out, csi+saveScreenSeq)
}

// AltScreen switches to the alternate screen buffer. The former view can be
// restored with ExitAltScreen().
func (o *Output) AltScreen() {
	fmt.Fprint(o.out, csi+altScreenSeq)
}

// ExitAltScreen exits the alternate screen buffer and returns to the former
// terminal view.
func (o *Output) ExitAltScreen() {
	fmt.Fprint(o.out, csi+exitAltScreenSeq)
}

// ClearScreen clears the visible portion of the terminal.
func (o *Output) ClearScreen() {
	fmt.Fprintf(o.out, csi+eraseDisplaySeq, clearScreen)
	o.MoveCursor(1, 1)
}

// MoveCursor moves the cursor to a given position.
func (o *Output) MoveCursor(row, column int) {
	fmt.Fprintf(o.out, csi+cursorPositionSeq, row, column)
}

// HideCursor hides the cursor.
func (o *Output) HideCursor() {
	fmt.Fprint(o.out, csi+hideCursorSeq)
}

// ShowCursor shows the cursor.
func (o *Output) ShowCursor() {
	fmt.Fprint(o.out, csi+showCursorSeq)
}

// SaveCursorPosition saves the cursor position.
func (o *Output) SaveCursorPosition() {
	fmt.Fprint(o.out, csi+saveCursorPositionSeq)
}

// RestoreCursorPosition restores a saved cursor position.
func (o *Output) RestoreCursorPosition() {
	fmt.Fprint(o.out, csi+restoreCursorPositionSeq)
}

// CursorUp moves the cursor up a given number of lines.
func (o *Output) CursorUp(n int) {
	fmt.Fprintf(o.out, csi+cursorUpSeq, n)
}

// CursorDown moves the cursor down a given number of lines.
func (o *Output) CursorDown(n int) {
	fmt.Fprintf(o.out, csi+cursorDownSeq, n)
}

// CursorForward moves the cursor up a given number of lines.
func (o *Output) CursorForward(n int) {
	fmt.Fprintf(o.out, csi+cursorForwardSeq, n)
}

// CursorBack moves the cursor backwards a given number of cells.
func (o *Output) CursorBack(n int) {
	fmt.Fprintf(o.out, csi+cursorBackSeq, n)
}

// CursorNextLine moves the cursor down a given number of lines and places it at
// the beginning of the line.
func (o *Output) CursorNextLine(n int) {
	fmt.Fprintf(o.out, csi+cursorNextLineSeq, n)
}

// CursorPrevLine moves the cursor up a given number of lines and places it at
// the beginning of the line.
func (o *Output) CursorPrevLine(n int) {
	fmt.Fprintf(o.out, csi+cursorPreviousLineSeq, n)
}

// ClearLine clears the current line.
func (o *Output) ClearLine() {
	fmt.Fprint(o.out, csi+eraseEntireLineSeq)
}

// ClearLineLeft clears the line to the left of the cursor.
func (o *Output) ClearLineLeft() {
	fmt.Fprint(o.out, csi+eraseLineLeftSeq)
}

// ClearLineRight clears the line to the right of the cursor.
func (o *Output) ClearLineRight() {
	fmt.Fprint(o.out, csi+eraseLineRightSeq)
}

// ClearLines clears a given number of lines.
func (o *Output) ClearLines(n int) {
	clearLine := fmt.Sprintf(csi+eraseLineSeq, clearScreen)
	cursorUp := fmt.Sprintf(csi+cursorUpSeq, moveCursorUp)

	fmt.Fprint(o.out, clearLine+strings.Repeat(cursorUp+clearLine, n))
}

// ChangeScrollingRegion sets the scrolling region of the terminal.
func (o *Output) ChangeScrollingRegion(top, bottom int) {
	fmt.Fprintf(o.out, csi+changeScrollingRegionSeq, top, bottom)
}

// InsertLines inserts the given number of lines at the top of the scrollable
// region, pushing lines below down.
func (o *Output) InsertLines(n int) {
	fmt.Fprintf(o.out, csi+insertLineSeq, n)
}

// DeleteLines deletes the given number of lines, pulling any lines in
// the scrollable region below up.
func (o *Output) DeleteLines(n int) {
	fmt.Fprintf(o.out, csi+deleteLineSeq, n)
}

// SetWindowTitle sets the terminal window title.
func (o *Output) SetWindowTitle(title string) {
	fmt.Fprintf(o.out, osc+setWindowTitleSeq, title)
}
