package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Setup creates a JSON logger that writes to ~/.looper/debug.log.
// It returns the logger, a cleanup function to close the log file, and any error.
// The log file is truncated on each session so it reflects only the current run.
func Setup() (*slog.Logger, func() error, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("getting home directory: %w", err)
	}

	dir := filepath.Join(home, ".looper")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating log directory: %w", err)
	}

	logPath := filepath.Join(dir, "debug.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("opening log file: %w", err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	return logger, f.Close, nil
}
