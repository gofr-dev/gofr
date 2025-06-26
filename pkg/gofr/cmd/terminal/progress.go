package terminal

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	errTermSize     = errors.New("error getting terminal size, could not initialize progress bar")
	errInvalidTotal = errors.New("error initializing progress bar, total should be > 0")
)

type ProgressBar struct {
	stream  Output
	current int64
	total   int64
	tWidth  int
	mu      sync.Mutex
}

type Term interface {
	IsTerminal(fd int) bool
	GetSize(fd int) (width, height int, err error)
}

func NewProgressBar(out Output, total int64) (*ProgressBar, error) {
	p := &ProgressBar{
		stream:  out,
		total:   total,
		tWidth:  0,
		current: 0,
	}

	w, _, err := out.getSize()
	if err != nil {
		return p, errTermSize
	}

	if total < 0 {
		p.total = 0

		return p, errInvalidTotal
	}

	p.tWidth = w

	return p, nil
}

func (p *ProgressBar) Incr(i int64) bool {
	// acquiring locks to synchronize the painting of the progress bar
	// and incrementing the current progress by the increment value.
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.current < p.total {
		p.current += i
		p.current = min(p.current, p.total)

		p.updateProgressBar()
	}

	return p.current != p.total
}

func (p *ProgressBar) updateProgressBar() {
	// perform the TUI update of the progress bar
	p.stream.Print("\r")

	pString := p.getString()
	p.stream.Print(pString)

	if p.current >= p.total {
		p.stream.Print("\n")
	}
}

const (
	// max rounded percentage.
	maxRP = 50
	// minimum terminal width required to render a progress bar.
	minTermWidth = 110
)

func (p *ProgressBar) getString() string {
	if p.current <= 0 && p.total <= 0 {
		return ""
	}

	percentage := float64(p.current) / float64(p.total) * 100

	numbersBox := fmt.Sprintf("%.3f%c", percentage, '%')

	if p.tWidth < minTermWidth {
		return numbersBox
	}

	return getProgressBox(percentage) + numbersBox
}

func getProgressBox(percentage float64) string {
	var pbBox string

	roundedPercent := int(percentage) / 2
	numSpaces := 0

	if maxRP-roundedPercent > 0 {
		numSpaces = maxRP - roundedPercent
	}

	if roundedPercent > 0 && roundedPercent < 50 {
		pbBox = fmt.Sprintf("[%s%s%s] ", strings.Repeat("█", roundedPercent-1), "░", strings.Repeat(" ", numSpaces))
	} else if roundedPercent <= 0 {
		pbBox = fmt.Sprintf("[%s] ", strings.Repeat(" ", numSpaces))
	} else {
		pbBox = fmt.Sprintf("[%s%s] ", strings.Repeat("█", roundedPercent), strings.Repeat(" ", numSpaces))
	}

	return pbBox
}
