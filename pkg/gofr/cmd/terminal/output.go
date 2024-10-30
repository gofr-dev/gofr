package terminal

import (
	"io"
	"os"

	"golang.org/x/term"
)

// terminal stores the UNIX file descriptor and isTerminal check for the tty
type terminal struct {
	fd         uintptr
	isTerminal bool
}

// Output manages the cli output that is user facing with many functionalites
// to manage and control the TUI (Terminal User Interface)
type Output struct {
	terminal
	out io.Writer
}

type Out interface {
	AltScreen()
	ChangeScrollingRegion(top int, bottom int)
	ClearLine()
	ClearLineLeft()
	ClearLineRight()
	ClearLines(n int)
	ClearScreen()
	CursorBack(n int)
	CursorDown(n int)
	CursorForward(n int)
	CursorNextLine(n int)
	CursorPrevLine(n int)
	CursorUp(n int)
	DeleteLines(n int)
	ExitAltScreen()
	HideCursor()
	InsertLines(n int)
	MoveCursor(row int, column int)
	Print(messages ...interface{})
	Printf(format string, args ...interface{})
	Println(messages ...interface{})
	Reset()
	ResetColor()
	RestoreCursorPosition()
	RestoreScreen()
	SaveCursorPosition()
	SaveScreen()
	SetColor(colorCode int)
	SetWindowTitle(title string)
	ShowCursor()

	getSize() (int, int, error)
}

// NewOutput intialises the output type with output stream as standard out
// and the output stream properties like file descriptor and the output is a terminal
func New() *Output {
	o := &Output{out: os.Stdout}
	o.fd, o.isTerminal = getTerminalInfo(o.out)

	return o
}

func getTerminalInfo(in io.Writer) (inFd uintptr, isTerminalIn bool) {
	if file, ok := in.(*os.File); ok {
		inFd = file.Fd()
		isTerminalIn = term.IsTerminal(int(inFd))
	}

	return inFd, isTerminalIn
}

func (o *Output) getSize() (int, int, error) {
	width, height, err := term.GetSize(int(o.fd))
	if err != nil {
		return 0, 0, err
	}

	return width, height, nil
}
