package ui

import "testing"

func TestViewContext_UpdateTerminalSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width         int
		height        int
		wantWidth     int
		wantHeight    int
		wantContent   int
	}{
		{
			name:        "normal terminal size",
			width:       80,
			height:      24,
			wantWidth:   80,
			wantHeight:  24,
			wantContent: 20, // 24 - 4 (header + footer + input)
		},
		{
			name:        "large terminal",
			width:       120,
			height:      40,
			wantWidth:   120,
			wantHeight:  40,
			wantContent: 36, // 40 - 4
		},
		{
			name:        "small terminal",
			width:       60,
			height:      10,
			wantWidth:   60,
			wantHeight:  10,
			wantContent: 6, // 10 - 4
		},
		{
			name:        "very small terminal - minimum content height",
			width:       40,
			height:      4,
			wantWidth:   40,
			wantHeight:  4,
			wantContent: 1, // minimum is 1, not 0
		},
		{
			name:        "tiny terminal - still minimum content height",
			width:       20,
			height:      2,
			wantWidth:   20,
			wantHeight:  2,
			wantContent: 1, // minimum is 1, even when calculation would be negative
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh context for each test to avoid interference
			ctx := &ViewContext{}
			ctx.UpdateTerminalSize(tt.width, tt.height)

			if ctx.TerminalWidth != tt.wantWidth {
				t.Errorf("UpdateTerminalSize() TerminalWidth = %v, want %v", ctx.TerminalWidth, tt.wantWidth)
			}
			if ctx.TerminalHeight != tt.wantHeight {
				t.Errorf("UpdateTerminalSize() TerminalHeight = %v, want %v", ctx.TerminalHeight, tt.wantHeight)
			}
			if ctx.ContentHeight != tt.wantContent {
				t.Errorf("UpdateTerminalSize() ContentHeight = %v, want %v", ctx.ContentHeight, tt.wantContent)
			}
		})
	}
}

func TestGetViewContext(t *testing.T) {
	t.Parallel()

	ctx := GetViewContext()
	if ctx == nil {
		t.Fatal("GetViewContext() returned nil")
	}

	// Verify it returns the same instance on multiple calls
	ctx2 := GetViewContext()
	if ctx != ctx2 {
		t.Error("GetViewContext() should return the same instance")
	}

	// Verify initial values
	if ctx.TerminalWidth != 80 {
		t.Errorf("initial TerminalWidth = %v, want 80", ctx.TerminalWidth)
	}
	if ctx.TerminalHeight != 24 {
		t.Errorf("initial TerminalHeight = %v, want 24", ctx.TerminalHeight)
	}
	if ctx.ContentHeight != 20 {
		t.Errorf("initial ContentHeight = %v, want 20", ctx.ContentHeight)
	}
}

func TestViewContext_UpdateTerminalSize_GlobalContext(t *testing.T) {
	// This test intentionally does NOT use t.Parallel() because it modifies global state
	
	// Save original state
	originalWidth := viewContext.TerminalWidth
	originalHeight := viewContext.TerminalHeight
	originalContent := viewContext.ContentHeight

	// Restore original state after test
	defer func() {
		viewContext.TerminalWidth = originalWidth
		viewContext.TerminalHeight = originalHeight
		viewContext.ContentHeight = originalContent
	}()

	// Test updating the global context through GetViewContext
	ctx := GetViewContext()
	ctx.UpdateTerminalSize(100, 30)

	if ctx.TerminalWidth != 100 {
		t.Errorf("global context TerminalWidth = %v, want 100", ctx.TerminalWidth)
	}
	if ctx.TerminalHeight != 30 {
		t.Errorf("global context TerminalHeight = %v, want 30", ctx.TerminalHeight)
	}
	if ctx.ContentHeight != 26 {
		t.Errorf("global context ContentHeight = %v, want 26", ctx.ContentHeight)
	}

	// Verify the global context was actually updated
	ctx2 := GetViewContext()
	if ctx2.TerminalWidth != 100 {
		t.Error("global context was not properly updated")
	}
}

func TestViewContext_ContentHeightCalculation(t *testing.T) {
	t.Parallel()

	ctx := &ViewContext{}

	// Test the exact calculation: height - 4 (header + footer + input area)
	testCases := []struct {
		height      int
		wantContent int
	}{
		{height: 10, wantContent: 6},   // 10 - 4 = 6
		{height: 5, wantContent: 1},    // 5 - 4 = 1 (minimum)
		{height: 4, wantContent: 1},    // 4 - 4 = 0, but minimum is 1
		{height: 3, wantContent: 1},    // 3 - 4 = -1, but minimum is 1
		{height: 1, wantContent: 1},    // 1 - 4 = -3, but minimum is 1
		{height: 0, wantContent: 1},    // 0 - 4 = -4, but minimum is 1
	}

	for _, tc := range testCases {
		ctx.UpdateTerminalSize(80, tc.height)
		if ctx.ContentHeight != tc.wantContent {
			t.Errorf("height %d: ContentHeight = %v, want %v", tc.height, ctx.ContentHeight, tc.wantContent)
		}
	}
}