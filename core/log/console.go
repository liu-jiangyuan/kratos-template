package log

import (
	"io"
	"log/slog"
	"os"
)

// ConsoleLogger outputs formatted log records to a terminal stream (such as os.Stdout or os.Stderr).
type ConsoleLogger struct {
	*baseLogger
}

var _ Logger = (*ConsoleLogger)(nil)

// NewConsoleLogger creates a Logger that writes to out (defaults to os.Stdout if nil).
func NewConsoleLogger(out io.Writer, level slog.Level, formatter Formatter, addSource bool) *ConsoleLogger {
	if out == nil {
		out = os.Stdout
	}
	return &ConsoleLogger{
		baseLogger: newBaseLogger(out, nil, level, formatter, addSource),
	}
}
