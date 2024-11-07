package terminal

import (
	"context"
	"time"
)

// Spinner is a TUI component that displays a loading spinner which can be used to
// denote a running background process.
type Spinner struct {
	// frames denote the frames of the spinner that displays in continiuation
	frames []string
	// fps is the speed at which the spinner frames are displayed
	fps time.Duration
	// outStream is the Output stream to which the spinner frames are printed onto
	outStream Output

	// started denotes whether the spinner has started spinning and ticker
	// is the time.Ticker for the continuous time update for the spinner.
	started bool
	ticker  *time.Ticker
}

func NewDotSpinner(o Output) *Spinner {
	return &Spinner{
		frames:    []string{"⣾ ", "⣽ ", "⣻ ", "⢿ ", "⡿ ", "⣟ ", "⣯ ", "⣷ "},
		fps:       time.Second / 10,
		outStream: o,
	}
}

func NewPulseSpinner(o Output) *Spinner {
	return &Spinner{
		frames:    []string{"█", "▓", "▒", "░"},
		fps:       time.Second / 4,
		outStream: o,
	}
}

func NewGlobeSpinner(o Output) *Spinner {
	return &Spinner{
		frames:    []string{"🌍", "🌎", "🌏"},
		fps:       time.Second / 4,
		outStream: o,
	}
}

func (s *Spinner) Spin(ctx context.Context) *Spinner {
	t := time.NewTicker(s.fps)
	s.ticker = t
	s.started = true
	i := 0

	s.outStream.HideCursor()

	go func() {
		for range t.C {
			select {
			case <-ctx.Done():
				t.Stop()
			default:
				if !s.started {
					break
				}

				s.outStream.Print("\r")
				s.outStream.Printf("%s", s.frames[i%len(s.frames)])

				i++
			}
		}
	}()

	return s
}

func (s *Spinner) Stop() {
	s.started = false
	s.ticker.Stop()

	s.outStream.ClearLine()
	s.outStream.ShowCursor()
	s.outStream.CursorBack(1)
}
