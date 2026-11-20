package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/liu-jiangyuan/kratos-template/core/conf"

	"github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	kratoslog "github.com/go-kratos/kratos/v3/log"
)

const (
	// DriverConsole outputs log records to the terminal (stdout).
	DriverConsole = "console"
	// DriverFile appends log records to a local file.
	DriverFile = "file"
	// DriverDatabase ships log records via HTTP API to a log database (Elasticsearch, ZincSearch, etc.).
	DriverDatabase = "database"
)

// Logger defines the unified logging interface implemented by the console,
// file, and HTTP log-database drivers.
type Logger interface {
	slog.Handler
	io.Closer

	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
	Slog() *slog.Logger
}

type baseLogger struct {
	mu        sync.Mutex
	writer    io.Writer
	closer    io.Closer
	level     slog.Level
	formatter Formatter
	addSource bool
	preAttrs  []slog.Attr
	groups    []string
	slogger   *slog.Logger
}

func newBaseLogger(w io.Writer, closer io.Closer, level slog.Level, formatter Formatter, addSource bool) *baseLogger {
	if formatter == nil {
		formatter = NewFormatter("text", "")
	}
	l := &baseLogger{
		writer:    w,
		closer:    closer,
		level:     level,
		formatter: formatter,
		addSource: addSource,
	}
	l.slogger = kratoslog.NewLogger(
		l,
		kratoslog.WithExtractor(tracing.TraceAttrs),
	)
	return l
}

func (l *baseLogger) Enabled(_ context.Context, level slog.Level) bool {
	return level >= l.level
}

func (l *baseLogger) Handle(_ context.Context, r slog.Record) error {
	rec := r
	if !l.addSource {
		rec.PC = 0
	}
	data, err := l.formatter.Format(rec, l.preAttrs, l.groups)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err = l.writer.Write(data)
	return err
}

func (l *baseLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(l.preAttrs)+len(attrs))
	merged = append(merged, l.preAttrs...)
	merged = append(merged, attrs...)
	clone := &baseLogger{
		writer:    l.writer,
		closer:    l.closer,
		level:     l.level,
		formatter: l.formatter,
		addSource: l.addSource,
		preAttrs:  merged,
		groups:    append([]string(nil), l.groups...),
	}
	clone.slogger = slog.New(clone)
	return clone
}

func (l *baseLogger) WithGroup(name string) slog.Handler {
	if name == "" {
		return l
	}
	groups := make([]string, 0, len(l.groups)+1)
	groups = append(groups, l.groups...)
	groups = append(groups, name)
	clone := &baseLogger{
		writer:    l.writer,
		closer:    l.closer,
		level:     l.level,
		formatter: l.formatter,
		addSource: l.addSource,
		preAttrs:  append([]slog.Attr(nil), l.preAttrs...),
		groups:    groups,
	}
	clone.slogger = slog.New(clone)
	return clone
}

func (l *baseLogger) Debug(ctx context.Context, msg string, args ...any) {
	l.slogger.DebugContext(ctx, msg, args...)
}

func (l *baseLogger) Info(ctx context.Context, msg string, args ...any) {
	l.slogger.InfoContext(ctx, msg, args...)
}

func (l *baseLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.slogger.WarnContext(ctx, msg, args...)
}

func (l *baseLogger) Error(ctx context.Context, msg string, args ...any) {
	l.slogger.ErrorContext(ctx, msg, args...)
}

func (l *baseLogger) Slog() *slog.Logger {
	return l.slogger
}

func (l *baseLogger) Close() error {
	if l.closer != nil {
		return l.closer.Close()
	}
	return nil
}

// ParseLevel parses a level string ("debug", "info", "warn", "error") into slog.Level.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New builds the configured Logger implementation ("console", "file", or "database").
func New(c *conf.Log) (Logger, func(), error) {
	if c == nil {
		logger := NewConsoleLogger(os.Stdout, slog.LevelInfo, NewFormatter("text", ""), true)
		return logger, func() { _ = logger.Close() }, nil
	}
	level := ParseLevel(c.GetLevel())
	formatStr := strings.TrimSpace(c.GetFormat())
	driver := strings.ToLower(strings.TrimSpace(c.GetDriver()))
	if driver == "" {
		driver = DriverConsole
	}
	if formatStr == "" {
		if driver == DriverDatabase {
			formatStr = "json"
		} else {
			formatStr = "text"
		}
	}
	formatter := NewFormatter(formatStr, c.GetTimeFormat())

	var (
		logger Logger
		err    error
	)
	switch driver {
	case DriverConsole:
		logger = NewConsoleLogger(os.Stdout, level, formatter, c.GetAddSource())
	case DriverFile:
		path := c.GetFile().GetPath()
		if path == "" {
			path = "./logs/app.log"
		}
		logger, err = NewFileLogger(path, level, formatter, c.GetAddSource())
	case DriverDatabase:
		dbCfg := c.GetDatabase()
		var timeout = defaultHTTPTimeout
		if dbCfg.GetTimeout() != nil {
			timeout = dbCfg.GetTimeout().AsDuration()
		}
		logger, err = NewDatabaseLogger(DatabaseConfig{
			Endpoint: dbCfg.GetEndpoint(),
			Method:   dbCfg.GetMethod(),
			Headers:  dbCfg.GetHeaders(),
			Timeout:  timeout,
		}, level, formatter, c.GetAddSource())
	default:
		return nil, nil, fmt.Errorf("unsupported log driver %q (expected console, file, or database)", driver)
	}
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = logger.Close()
	}
	return logger, cleanup, nil
}
