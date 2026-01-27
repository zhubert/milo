package ui

import "sync"

// ViewContext holds terminal sizing information used by all components.
type ViewContext struct {
	TerminalWidth  int
	TerminalHeight int
	ContentHeight  int

	mu sync.Mutex
}

var (
	ctx     *ViewContext
	ctxOnce sync.Once
)

// GetViewContext returns the singleton ViewContext.
func GetViewContext() *ViewContext {
	ctxOnce.Do(func() {
		ctx = &ViewContext{}
	})
	return ctx
}

// UpdateTerminalSize recalculates layout dimensions.
func (v *ViewContext) UpdateTerminalSize(width, height int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.TerminalWidth = width
	v.TerminalHeight = height
	v.ContentHeight = height - HeaderHeight - FooterHeight
	if v.ContentHeight < 1 {
		v.ContentHeight = 1
	}
}
