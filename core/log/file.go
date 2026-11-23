package log

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// FileLogger appends formatted log records to a local file on disk.
// For long-running production services, pair this with external logrotate or
// swap the underlying writer for a rotating writer (e.g. lumberjack).
type FileLogger struct {
	*baseLogger
}

var _ Logger = (*FileLogger)(nil)

// NewFileLogger creates a Logger that writes to filePath, creating parent directories if needed.
func NewFileLogger(filePath string, level slog.Level, formatter Formatter, addSource bool) (*FileLogger, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file log path cannot be empty")
	}
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create log dir %q: %w", dir, err)
		}
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", filePath, err)
	}
	return &FileLogger{
		baseLogger: newBaseLogger(f, f, level, formatter, addSource),
	}, nil
}
