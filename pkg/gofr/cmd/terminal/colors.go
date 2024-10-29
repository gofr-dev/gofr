package terminal

import "strconv"

const (
	Black = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
	BrightBlack
	BrightRed
	BrightGreen
	BrightYellow
	BrightBlue
	BrightMagenta
	BrightCyan
	BrightWhite
)

func (o *Output) SetColor(colorCode int) {
	o.Printf(csi + "38;5;" + strconv.Itoa(colorCode) + "m")
}

func (o *Output) ResetColor() {
	o.Printf(csi + "0m")
}
