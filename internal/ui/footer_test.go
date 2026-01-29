package ui

import "testing"

func TestFooter_FlashMessages(t *testing.T) {
	t.Parallel()

	f := NewFooter()

	// Initially no flash
	if f.flash != "" {
		t.Error("expected empty flash initially")
	}

	// Set flash message
	f.SetFlash("test message")
	if f.flash != "test message" {
		t.Errorf("SetFlash() flash = %q, want %q", f.flash, "test message")
	}

	// Clear flash
	f.ClearFlash()
	if f.flash != "" {
		t.Error("ClearFlash() should set flash to empty string")
	}
}

func TestFooter_TokenUsage(t *testing.T) {
	t.Parallel()

	f := NewFooter()

	// Initially zero
	if f.TotalTokens() != 0 {
		t.Errorf("TotalTokens() = %d, want 0", f.TotalTokens())
	}

	// Add usage for one model
	f.AddUsage("claude-sonnet-4", 100, 50)
	if f.TotalTokens() != 150 {
		t.Errorf("TotalTokens() = %d, want 150", f.TotalTokens())
	}

	// Check per-model usage
	usage := f.UsageByModel()
	if usage["claude-sonnet-4"].InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", usage["claude-sonnet-4"].InputTokens)
	}
	if usage["claude-sonnet-4"].OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", usage["claude-sonnet-4"].OutputTokens)
	}

	// Add more usage for same model (should accumulate)
	f.AddUsage("claude-sonnet-4", 200, 100)
	if f.TotalTokens() != 450 {
		t.Errorf("TotalTokens() = %d, want 450", f.TotalTokens())
	}
	if usage["claude-sonnet-4"].InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", usage["claude-sonnet-4"].InputTokens)
	}
}

func TestFooter_TokenUsageMultipleModels(t *testing.T) {
	t.Parallel()

	f := NewFooter()

	// Add usage for different models
	f.AddUsage("claude-sonnet-4", 100, 50)
	f.AddUsage("claude-opus-4", 200, 100)

	// Total should include both
	if f.TotalTokens() != 450 {
		t.Errorf("TotalTokens() = %d, want 450", f.TotalTokens())
	}

	// Each model should have its own usage
	usage := f.UsageByModel()
	if usage["claude-sonnet-4"].InputTokens != 100 {
		t.Errorf("sonnet InputTokens = %d, want 100", usage["claude-sonnet-4"].InputTokens)
	}
	if usage["claude-opus-4"].InputTokens != 200 {
		t.Errorf("opus InputTokens = %d, want 200", usage["claude-opus-4"].InputTokens)
	}
}

func TestFormatTokenCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		count int64
		want  string
	}{
		{count: 0, want: "0"},
		{count: 100, want: "100"},
		{count: 999, want: "999"},
		{count: 1000, want: "1.0K"},
		{count: 1500, want: "1.5K"},
		{count: 10000, want: "10.0K"},
		{count: 100500, want: "100.5K"},
	}

	for _, tt := range tests {
		got := formatTokenCount(tt.count)
		if got != tt.want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}
