package loopdetector

import (
	"testing"
)

func TestNewWithDefaults(t *testing.T) {
	t.Parallel()

	d := NewWithDefaults()
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if d.Iterations() != 0 {
		t.Errorf("expected 0 iterations, got %d", d.Iterations())
	}
	if d.config.MaxIterations != DefaultMaxIterations {
		t.Errorf("expected max iterations %d, got %d", DefaultMaxIterations, d.config.MaxIterations)
	}
}

func TestDetector_RecordIteration(t *testing.T) {
	t.Parallel()

	d := NewWithDefaults()
	if d.Iterations() != 0 {
		t.Fatalf("expected 0 iterations initially, got %d", d.Iterations())
	}

	d.RecordIteration()
	if d.Iterations() != 1 {
		t.Errorf("expected 1 iteration, got %d", d.Iterations())
	}

	d.RecordIteration()
	d.RecordIteration()
	if d.Iterations() != 3 {
		t.Errorf("expected 3 iterations, got %d", d.Iterations())
	}
}

func TestDetector_MaxIterationsExceeded(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxIterations:        3,
		MaxRepeatedCalls:     10,
		MaxConsecutiveErrors: 10,
	}
	d := New(config)

	// First 3 iterations should be fine
	for i := 0; i < 3; i++ {
		d.RecordIteration()
		detection := d.Check()
		if detection.Detected {
			t.Errorf("iteration %d: unexpected doom loop detection: %s", i+1, detection.Reason)
		}
	}

	// 4th iteration should trigger detection
	d.RecordIteration()
	detection := d.Check()
	if !detection.Detected {
		t.Error("expected doom loop detection after exceeding max iterations")
	}
	if detection.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDetector_RepeatedToolCalls(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxIterations:        100,
		MaxRepeatedCalls:     3,
		MaxConsecutiveErrors: 100,
	}
	d := New(config)

	// Same tool with same input called 2 times should be fine
	d.RecordToolCall("read", `{"path": "/etc/passwd"}`, "file contents", false)
	d.RecordToolCall("read", `{"path": "/etc/passwd"}`, "file contents", false)
	detection := d.Check()
	if detection.Detected {
		t.Errorf("unexpected doom loop detection after 2 calls: %s", detection.Reason)
	}

	// 3rd identical call should trigger detection
	d.RecordToolCall("read", `{"path": "/etc/passwd"}`, "file contents", false)
	detection = d.Check()
	if !detection.Detected {
		t.Error("expected doom loop detection after 3 identical calls")
	}
	if detection.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDetector_DifferentToolCalls(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxIterations:        100,
		MaxRepeatedCalls:     2,
		MaxConsecutiveErrors: 100,
	}
	d := New(config)

	// Different tools should not trigger detection
	d.RecordToolCall("read", `{"path": "/etc/passwd"}`, "file contents", false)
	d.RecordToolCall("write", `{"path": "/etc/passwd"}`, "ok", false)
	d.RecordToolCall("bash", `{"command": "ls"}`, "listing", false)

	detection := d.Check()
	if detection.Detected {
		t.Errorf("unexpected doom loop detection for different tools: %s", detection.Reason)
	}
}

func TestDetector_DifferentInputs(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxIterations:        100,
		MaxRepeatedCalls:     2,
		MaxConsecutiveErrors: 100,
	}
	d := New(config)

	// Same tool with different inputs should not trigger detection
	d.RecordToolCall("read", `{"path": "/etc/passwd"}`, "file1", false)
	d.RecordToolCall("read", `{"path": "/etc/hosts"}`, "file2", false)
	d.RecordToolCall("read", `{"path": "/etc/shadow"}`, "file3", false)

	detection := d.Check()
	if detection.Detected {
		t.Errorf("unexpected doom loop detection for different inputs: %s", detection.Reason)
	}
}

func TestDetector_ConsecutiveErrors(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxIterations:        100,
		MaxRepeatedCalls:     100,
		MaxConsecutiveErrors: 3,
	}
	d := New(config)

	// 2 consecutive identical errors should be fine
	d.RecordToolCall("bash", `{"command": "fail"}`, "command failed: exit 1", true)
	d.RecordToolCall("bash", `{"command": "fail2"}`, "command failed: exit 1", true)
	detection := d.Check()
	if detection.Detected {
		t.Errorf("unexpected doom loop detection after 2 errors: %s", detection.Reason)
	}

	// 3rd identical error should trigger detection
	d.RecordToolCall("bash", `{"command": "fail3"}`, "command failed: exit 1", true)
	detection = d.Check()
	if !detection.Detected {
		t.Error("expected doom loop detection after 3 consecutive identical errors")
	}
}

func TestDetector_ConsecutiveErrors_Reset(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxIterations:        100,
		MaxRepeatedCalls:     100,
		MaxConsecutiveErrors: 3,
	}
	d := New(config)

	// 2 identical errors, then success, then 2 more identical errors
	d.RecordToolCall("bash", `{"command": "fail"}`, "error1", true)
	d.RecordToolCall("bash", `{"command": "fail"}`, "error1", true)
	d.RecordToolCall("bash", `{"command": "work"}`, "success", false) // Success resets counter
	d.RecordToolCall("bash", `{"command": "fail"}`, "error1", true)
	d.RecordToolCall("bash", `{"command": "fail"}`, "error1", true)

	detection := d.Check()
	if detection.Detected {
		t.Errorf("unexpected doom loop detection when errors were interrupted by success: %s", detection.Reason)
	}
}

func TestDetector_Reset(t *testing.T) {
	t.Parallel()

	d := NewWithDefaults()

	// Add some state
	d.RecordIteration()
	d.RecordIteration()
	d.RecordToolCall("read", `{"path": "/etc/passwd"}`, "contents", false)
	d.RecordToolCall("read", `{"path": "/etc/passwd"}`, "contents", false)
	d.RecordToolCall("bash", `{"command": "fail"}`, "error", true)

	if d.Iterations() != 2 {
		t.Errorf("expected 2 iterations before reset, got %d", d.Iterations())
	}

	// Reset
	d.Reset()

	if d.Iterations() != 0 {
		t.Errorf("expected 0 iterations after reset, got %d", d.Iterations())
	}

	// Should be able to use again without triggering previous state
	d.RecordToolCall("read", `{"path": "/etc/passwd"}`, "contents", false)
	detection := d.Check()
	if detection.Detected {
		t.Errorf("unexpected detection after reset: %s", detection.Reason)
	}
}

func TestDetector_NoDetection(t *testing.T) {
	t.Parallel()

	d := NewWithDefaults()

	// Normal usage: some iterations and varied tool calls
	for i := 0; i < 5; i++ {
		d.RecordIteration()
		d.RecordToolCall("read", `{"path": "/file`+string(rune('a'+i))+`"}`, "content", false)
	}

	detection := d.Check()
	if detection.Detected {
		t.Errorf("unexpected doom loop detection in normal usage: %s", detection.Reason)
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	if config.MaxIterations != DefaultMaxIterations {
		t.Errorf("expected MaxIterations %d, got %d", DefaultMaxIterations, config.MaxIterations)
	}
	if config.MaxRepeatedCalls != DefaultMaxRepeatedCalls {
		t.Errorf("expected MaxRepeatedCalls %d, got %d", DefaultMaxRepeatedCalls, config.MaxRepeatedCalls)
	}
	if config.MaxConsecutiveErrors != DefaultMaxConsecutiveErrors {
		t.Errorf("expected MaxConsecutiveErrors %d, got %d", DefaultMaxConsecutiveErrors, config.MaxConsecutiveErrors)
	}
}
