package log

import (
	"bytes"
	"io"
	"log/slog"
	"os"
)

const (
	// ANSI color escape sequences for terminal log output.
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"
)

// ColorForLevel returns the ANSI color escape code for a given slog.Level when
// outputting to a terminal:
//   - DEBUG: "" (terminal default character color)
//   - INFO:  blue ("\x1b[34m")
//   - WARN:  yellow ("\x1b[33m")
//   - ERROR: red ("\x1b[31m")
func ColorForLevel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return ansiRed
	case level >= slog.LevelWarn:
		return ansiYellow
	case level >= slog.LevelInfo:
		return ansiBlue
	default:
		return ""
	}
}

// ColorizeLine wraps a formatted log line with the ANSI color for level,
// keeping debug-level lines in the default terminal color.
func ColorizeLine(level slog.Level, line []byte) []byte {
	color := ColorForLevel(level)
	if color == "" || len(line) == 0 {
		return line
	}
	hasNewline := line[len(line)-1] == '\n'
	content := line
	if hasNewline {
		content = line[:len(line)-1]
	}
	var buf bytes.Buffer
	buf.Grow(len(color) + len(content) + len(ansiReset) + 1)
	buf.WriteString(color)
	buf.Write(content)
	buf.WriteString(ansiReset)
	if hasNewline {
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// ConsoleLogger outputs formatted log records to a terminal stream (such as os.Stdout or os.Stderr).
type ConsoleLogger struct {
	*baseLogger
}

var _ Logger = (*ConsoleLogger)(nil)

// NewConsoleLogger creates a Logger that writes to out (defaults to os.Stdout if nil)
// with level-based terminal ANSI coloring enabled.
func NewConsoleLogger(out io.Writer, level slog.Level, formatter Formatter, addSource bool) *ConsoleLogger {
	if out == nil {
		out = os.Stdout
	}
	base := newBaseLogger(out, nil, level, formatter, addSource)
	base.colorize = true
	return &ConsoleLogger{
		baseLogger: base,
	}
}
