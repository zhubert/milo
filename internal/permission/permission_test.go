package permission

import "testing"

func TestDefaultRules(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	tests := []struct {
		tool   string
		expect Action
	}{
		{"read", Allow},
		{"write", Ask},
		{"edit", Ask},
		{"bash", Ask},
		{"unknown", Ask},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			t.Parallel()
			got := c.Check(tt.tool)
			if got != tt.expect {
				t.Errorf("Check(%q) = %d, want %d", tt.tool, got, tt.expect)
			}
		})
	}
}

func TestAllowAlways(t *testing.T) {
	t.Parallel()

	c := NewChecker()

	// Before AllowAlways, bash requires ask.
	if got := c.Check("bash"); got != Ask {
		t.Fatalf("expected Ask for bash, got %d", got)
	}

	c.AllowAlways("bash")

	// After AllowAlways, bash should be allowed.
	if got := c.Check("bash"); got != Allow {
		t.Fatalf("expected Allow for bash after AllowAlways, got %d", got)
	}
}

func TestAllowAlwaysPersists(t *testing.T) {
	t.Parallel()

	c := NewChecker()
	c.AllowAlways("write")

	// Check multiple times — should remain Allow.
	for i := 0; i < 3; i++ {
		if got := c.Check("write"); got != Allow {
			t.Errorf("check %d: expected Allow, got %d", i, got)
		}
	}
}

func TestSetRule(t *testing.T) {
	t.Parallel()

	c := NewChecker()
	c.SetRule("read", Deny)

	if got := c.Check("read"); got != Deny {
		t.Errorf("expected Deny after SetRule, got %d", got)
	}
}
