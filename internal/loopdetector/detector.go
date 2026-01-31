// Package loopdetector provides doom loop detection for the agent.
// It tracks tool call patterns and detects when the agent is stuck
// repeating the same actions without making progress.
package loopdetector

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Default thresholds for doom loop detection.
const (
	DefaultMaxIterations        = 200 // Maximum turns per user request
	DefaultMaxRepeatedCalls     = 3   // Same tool+input called this many times
	DefaultMaxConsecutiveErrors = 5   // Consecutive error results
)

// ToolCall represents a single tool invocation.
type ToolCall struct {
	Name    string
	Input   string
	Result  string
	IsError bool
}

// Config holds the thresholds for doom loop detection.
type Config struct {
	MaxIterations        int
	MaxRepeatedCalls     int
	MaxConsecutiveErrors int
}

// DefaultConfig returns the default detection configuration.
func DefaultConfig() Config {
	return Config{
		MaxIterations:        DefaultMaxIterations,
		MaxRepeatedCalls:     DefaultMaxRepeatedCalls,
		MaxConsecutiveErrors: DefaultMaxConsecutiveErrors,
	}
}

// Detection represents a doom loop detection result.
type Detection struct {
	Detected bool
	Reason   string
}

// Detector tracks tool call patterns and detects doom loops.
type Detector struct {
	config            Config
	iterations        int
	callHistory       []ToolCall
	inputHashes       map[string]int // hash(tool+input) -> count
	consecutiveErrors int
	lastErrorHash     string
}

// New creates a new Detector with the given configuration.
func New(config Config) *Detector {
	return &Detector{
		config:      config,
		inputHashes: make(map[string]int),
	}
}

// NewWithDefaults creates a new Detector with default configuration.
func NewWithDefaults() *Detector {
	return New(DefaultConfig())
}

// RecordIteration records that a new agent loop iteration has started.
// Call this at the beginning of each loop iteration.
func (d *Detector) RecordIteration() {
	d.iterations++
}

// RecordToolCall records a tool call and its result.
// Call this after each tool execution.
func (d *Detector) RecordToolCall(name, input, result string, isError bool) {
	call := ToolCall{
		Name:    name,
		Input:   input,
		Result:  result,
		IsError: isError,
	}
	d.callHistory = append(d.callHistory, call)

	// Track input hash for repetition detection
	hash := hashToolCall(name, input)
	d.inputHashes[hash]++

	// Track consecutive errors
	if isError {
		errHash := hashToolCall(name, result)
		if errHash == d.lastErrorHash {
			d.consecutiveErrors++
		} else {
			d.consecutiveErrors = 1
			d.lastErrorHash = errHash
		}
	} else {
		d.consecutiveErrors = 0
		d.lastErrorHash = ""
	}
}

// Check evaluates the current state and returns a Detection result.
// Call this after recording tool calls to see if a doom loop is detected.
func (d *Detector) Check() Detection {
	// Check iteration limit
	if d.iterations >= d.config.MaxIterations {
		return Detection{
			Detected: true,
			Reason:   fmt.Sprintf("exceeded maximum iterations (%d)", d.config.MaxIterations),
		}
	}

	// Check for repeated identical calls
	for hash, count := range d.inputHashes {
		if count >= d.config.MaxRepeatedCalls {
			// Find the tool name for better error message
			toolName := d.findToolNameForHash(hash)
			return Detection{
				Detected: true,
				Reason:   fmt.Sprintf("tool %q called %d times with identical input", toolName, count),
			}
		}
	}

	// Check consecutive error threshold
	if d.consecutiveErrors >= d.config.MaxConsecutiveErrors {
		return Detection{
			Detected: true,
			Reason:   fmt.Sprintf("received %d consecutive identical errors", d.consecutiveErrors),
		}
	}

	return Detection{Detected: false}
}

// Iterations returns the current iteration count.
func (d *Detector) Iterations() int {
	return d.iterations
}

// Reset clears all state. Call this when starting a new user request.
func (d *Detector) Reset() {
	d.iterations = 0
	d.callHistory = nil
	d.inputHashes = make(map[string]int)
	d.consecutiveErrors = 0
	d.lastErrorHash = ""
}

// findToolNameForHash finds the tool name associated with a hash.
func (d *Detector) findToolNameForHash(targetHash string) string {
	for _, call := range d.callHistory {
		if hashToolCall(call.Name, call.Input) == targetHash {
			return call.Name
		}
	}
	return "unknown"
}

// hashToolCall creates a hash of tool name and input for comparison.
func hashToolCall(name, input string) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
