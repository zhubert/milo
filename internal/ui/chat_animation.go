package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// StopwatchTickMsg advances the spinner animation.
type StopwatchTickMsg time.Time

// StopwatchTick returns a command that ticks the spinner animation.
func StopwatchTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return StopwatchTickMsg(t)
	})
}

// SpinnerState tracks the animation frame.
type SpinnerState struct {
	Idx     int
	Tick    int
	Started time.Time
}

// NewSpinnerState creates a new spinner starting now.
func NewSpinnerState() *SpinnerState {
	return &SpinnerState{Started: time.Now()}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Advance moves the spinner to the next frame.
func (s *SpinnerState) Advance() {
	s.Tick++
	if s.Tick >= 1 {
		s.Tick = 0
		s.Idx++
	}
}

// Frame returns the current spinner character.
func (s *SpinnerState) Frame() string {
	return spinnerFrames[s.Idx%len(spinnerFrames)]
}

// Elapsed returns the time since the spinner started.
func (s *SpinnerState) Elapsed() time.Duration {
	return time.Since(s.Started)
}

// RenderSpinner renders the spinner with a verb and elapsed time.
func (s *SpinnerState) RenderSpinner(verb string) string {
	elapsed := s.Elapsed().Truncate(time.Second)
	frame := ToolNameStyle.Render(s.Frame())
	label := DimStyle.Render(fmt.Sprintf(" %s... %s", verb, elapsed))
	return frame + label
}
