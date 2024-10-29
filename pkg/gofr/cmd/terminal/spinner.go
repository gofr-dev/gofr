package terminal

import (
	"time"
)

// Spinner is a TUI component that displays a loading spinner which can be used to
// denote a running background process.
type Spinner struct {
	// Frames denote the farmes of the spinner that displays in continiuation
	Frames []string
	// FPS is the speed at which the spinner Frames are displayed
	FPS time.Duration
	// Stream is the Output stream to which the spinner Frames are printed onto
	Stream Out

	// unexported started denotes whether the spinner has started spinning and ticker
	// is the time.Ticker for the continious time update for the spinner.
	started bool
	ticker  *time.Ticker
}

func NewDotSpinner(o Out) *Spinner {
	return &Spinner{
		Frames: []string{"⣾ ", "⣽ ", "⣻ ", "⢿ ", "⡿ ", "⣟ ", "⣯ ", "⣷ "},
		FPS:    time.Second / 10,
		Stream: o,
	}
}

func NewPulseSpinner(o Out) *Spinner {
	return &Spinner{
		Frames: []string{"█", "▓", "▒", "░"},
		FPS:    time.Second / 4,
		Stream: o,
	}
}

func NewGlobeSpinner(o Out) *Spinner {
	return &Spinner{
		Frames: []string{"🌍", "🌎", "🌏"},
		FPS:    time.Second / 4,
		Stream: o,
	}
}

func (s *Spinner) Spin() *Spinner {
	t := time.NewTicker(s.FPS)
	s.ticker = t
	s.started = true
	i := 0

	s.Stream.HideCursor()

	go func() {
		for range t.C {
			if s.started {
				s.Stream.Print("\r")
			} else {
				break
			}

			s.Stream.Printf("%s"+"", s.Frames[i%len(s.Frames)])

			i++
		}
	}()

	s.Stream.ClearLine()

	return s
}

func (s *Spinner) Stop() {
	s.started = false
	s.ticker.Stop()

	s.Stream.ClearLine()
	s.Stream.ShowCursor()
	s.Stream.CursorBack(1)
}
