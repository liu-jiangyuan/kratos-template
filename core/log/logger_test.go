package log

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liu-jiangyuan/kratos-template/core/conf"

	kratoslog "github.com/go-kratos/kratos/v3/log"
)

func TestConsoleLoggerAndCustomFormat(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewFormatter(
		"{time} [{level}] [req={request_id}] {msg} | {attrs}",
		"2006-01-02",
	)
	logger := NewConsoleLogger(&buf, slog.LevelDebug, formatter, false)
	defer logger.Close()

	ctx := kratoslog.ContextWithAttrs(context.Background(), slog.String("request_id", "req-001"))
	logger.Info(ctx, "user login", "user", "alice", "attempt", 1)

	out := buf.String()
	if !strings.Contains(out, "[INFO] [req=req-001] user login | user=alice attempt=1") {
		t.Fatalf("unexpected custom formatted log output: %q", out)
	}
	if !strings.HasPrefix(out, ansiBlue) || !strings.HasSuffix(out, ansiReset+"\n") {
		t.Fatalf("expected blue ANSI color wrapper on info console log, got %q", out)
	}
}

func TestConsoleLoggerDefaultJSONAndLevelColors(t *testing.T) {
	var buf bytes.Buffer
	// Write to both &buf (for assertions) and os.Stdout (so running `go test -v` displays colored logs in terminal).
	outWriter := io.MultiWriter(&buf, os.Stdout)
	// nil formatter and empty formatSpec both default to JSON mode.
	logger := NewConsoleLogger(outWriter, slog.LevelDebug, NewFormatter("", ""), true)
	defer logger.Close()

	ctx := context.Background()

	// 1. DEBUG: default terminal color (no ANSI escape prefix/suffix)
	buf.Reset()
	logger.Debug(ctx, "debug message", "k", "v")
	debugOut := buf.String()
	if strings.Contains(debugOut, "\x1b[") {
		t.Fatalf("expected debug log to use default terminal color without ANSI codes, got %q", debugOut)
	}
	if !strings.HasPrefix(debugOut, "[DEBUG] ") {
		t.Fatalf("expected debug log to start with '[DEBUG] ', got %q", debugOut)
	}
	debugJSON := strings.TrimPrefix(debugOut, "[DEBUG] ")
	var debugDoc map[string]any
	if err := json.Unmarshal([]byte(debugJSON), &debugDoc); err != nil {
		t.Fatalf("expected debug log payload to be valid JSON, err = %v, raw = %q", err, debugJSON)
	}
	if debugDoc["msg"] != "debug message" || debugDoc["k"] != "v" {
		t.Fatalf("unexpected debug JSON content: %+v", debugDoc)
	}
	if _, hasLevel := debugDoc["level"]; hasLevel {
		t.Fatalf("expected level to be in prefix [DEBUG] rather than inside JSON object: %+v", debugDoc)
	}
	if ts, ok := debugDoc["time"].(string); !ok {
		t.Fatalf("expected time string in JSON log, got %v", debugDoc["time"])
	} else if _, err := time.Parse(DefaultTimeFormat, ts); err != nil {
		t.Fatalf("expected time %q to match layout %q: %v", ts, DefaultTimeFormat, err)
	}

	// 2. INFO: blue (\x1b[34m[INFO] {...}\x1b[0m\n)
	buf.Reset()
	logger.Info(ctx, "info message")
	infoOut := buf.String()
	if !strings.HasPrefix(infoOut, ansiBlue+"[INFO] ") || !strings.HasSuffix(infoOut, ansiReset+"\n") {
		t.Fatalf("expected info log to be wrapped in blue with [INFO] prefix, got %q", infoOut)
	}
	infoJSON := strings.TrimSuffix(strings.TrimPrefix(infoOut, ansiBlue+"[INFO] "), ansiReset+"\n")
	var infoDoc map[string]any
	if err := json.Unmarshal([]byte(infoJSON), &infoDoc); err != nil {
		t.Fatalf("expected info log inner payload to be valid JSON, err = %v, raw = %q", err, infoJSON)
	}
	if infoDoc["msg"] != "info message" {
		t.Fatalf("unexpected info JSON content: %+v", infoDoc)
	}

	// 3. WARN: yellow (\x1b[33m[WARN] {...}\x1b[0m\n)
	buf.Reset()
	logger.Warn(ctx, "warn message")
	warnOut := buf.String()
	if !strings.HasPrefix(warnOut, ansiYellow+"[WARN] ") || !strings.HasSuffix(warnOut, ansiReset+"\n") {
		t.Fatalf("expected warn log to be wrapped in yellow with [WARN] prefix, got %q", warnOut)
	}
	warnJSON := strings.TrimSuffix(strings.TrimPrefix(warnOut, ansiYellow+"[WARN] "), ansiReset+"\n")
	var warnDoc map[string]any
	if err := json.Unmarshal([]byte(warnJSON), &warnDoc); err != nil {
		t.Fatalf("expected warn log inner payload to be valid JSON, err = %v, raw = %q", err, warnJSON)
	}
	if warnDoc["msg"] != "warn message" {
		t.Fatalf("unexpected warn JSON content: %+v", warnDoc)
	}

	// 4. ERROR: red (\x1b[31m[ERROR] {...}\x1b[0m\n)
	buf.Reset()
	logger.Error(ctx, "error message")
	errOut := buf.String()
	if !strings.HasPrefix(errOut, ansiRed+"[ERROR] ") || !strings.HasSuffix(errOut, ansiReset+"\n") {
		t.Fatalf("expected error log to be wrapped in red with [ERROR] prefix, got %q", errOut)
	}
	errJSON := strings.TrimSuffix(strings.TrimPrefix(errOut, ansiRed+"[ERROR] "), ansiReset+"\n")
	var errDoc map[string]any
	if err := json.Unmarshal([]byte(errJSON), &errDoc); err != nil {
		t.Fatalf("expected error log inner payload to be valid JSON, err = %v, raw = %q", err, errJSON)
	}
	if errDoc["msg"] != "error message" {
		t.Fatalf("unexpected error JSON content: %+v", errDoc)
	}

	// 5. Derived slog.Logger via .With(...) preserves terminal colorization and JSON mode
	buf.Reset()
	child := logger.Slog().With(slog.String("service.name", "demo"))
	child.ErrorContext(ctx, "child error")
	childOut := buf.String()
	if !strings.HasPrefix(childOut, ansiRed) || !strings.HasSuffix(childOut, ansiReset+"\n") {
		t.Fatalf("expected derived slog.Logger to preserve red color, got %q", childOut)
	}
}

func TestNewDefaultsToConsoleJSON(t *testing.T) {
	logger, cleanup, err := New(&conf.Log{})
	if err != nil {
		t.Fatalf("New(&conf.Log{}) error = %v", err)
	}
	defer cleanup()

	cl, ok := logger.(*ConsoleLogger)
	if !ok {
		t.Fatalf("expected *ConsoleLogger by default, got %T", logger)
	}
	if _, isJSON := cl.formatter.(*jsonFormatter); !isJSON {
		t.Fatalf("expected *jsonFormatter by default, got %T", cl.formatter)
	}
	if !cl.colorize {
		t.Fatal("expected colorize=true on default ConsoleLogger")
	}
}

func TestFileLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs", "service.log")

	logger, err := NewFileLogger(logPath, slog.LevelInfo, NewFormatter("text", ""), false)
	if err != nil {
		t.Fatalf("NewFileLogger() error = %v", err)
	}
	ctx := kratoslog.ContextWithAttrs(context.Background(), slog.String("request_id", "req-file-99"))
	logger.Info(ctx, "written to file", "module", "todo")
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, `msg="written to file"`) || !strings.Contains(content, "request_id=req-file-99") {
		t.Fatalf("unexpected file log content: %q", content)
	}

	// FileLogger in JSON mode outputs standard single-line JSON with "level" key-value (no "[INFO] " prefix).
	jsonPath := filepath.Join(tmpDir, "logs", "service.json.log")
	jsonLogger, err := NewFileLogger(jsonPath, slog.LevelInfo, NewFormatter("json", ""), false)
	if err != nil {
		t.Fatalf("NewFileLogger(json) error = %v", err)
	}
	jsonLogger.Info(ctx, "json to file", "module", "todo")
	if err := jsonLogger.Close(); err != nil {
		t.Fatalf("Close(json) error = %v", err)
	}
	jsonRaw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile(json) error = %v", err)
	}
	var fileDoc map[string]any
	if err := json.Unmarshal(jsonRaw, &fileDoc); err != nil {
		t.Fatalf("expected file JSON log to be pure valid JSON, err = %v, raw = %q", err, string(jsonRaw))
	}
	if fileDoc["level"] != "INFO" || fileDoc["msg"] != "json to file" || fileDoc["request_id"] != "req-file-99" {
		t.Fatalf("unexpected file JSON payload: %+v", fileDoc)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDatabaseLoggerHTTP(t *testing.T) {
	var (
		mu       sync.Mutex
		received []map[string]any
		authHdr  string
	)
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			var doc map[string]any
			if err := json.Unmarshal(body, &doc); err != nil {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			mu.Lock()
			authHdr = r.Header.Get("Authorization")
			received = append(received, doc)
			mu.Unlock()
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		}),
	}

	logger, err := NewDatabaseLogger(DatabaseConfig{
		Endpoint: "http://127.0.0.1:4080/api/logs/_doc",
		Method:   http.MethodPost,
		Headers: map[string]string{
			"Authorization": "Basic dGVzdDp0ZXN0",
		},
		Client: client,
	}, slog.LevelInfo, NewFormatter("json", ""), false)
	if err != nil {
		t.Fatalf("NewDatabaseLogger() error = %v", err)
	}
	defer logger.Close()

	ctx := kratoslog.ContextWithAttrs(context.Background(), slog.String("request_id", "req-db-777"))
	logger.Warn(ctx, "slow query detected", "cost_ms", 320)
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 HTTP log payload, got %d", len(received))
	}
	if authHdr != "Basic dGVzdDp0ZXN0" {
		t.Fatalf("Authorization header = %q, want Basic dGVzdDp0ZXN0", authHdr)
	}
	doc := received[0]
	if doc["msg"] != "slow query detected" || doc["level"] != "WARN" || doc["request_id"] != "req-db-777" {
		t.Fatalf("unexpected JSON payload sent to database API: %+v", doc)
	}
}

func TestDerivedLoggersShareMutex(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf, slog.LevelInfo, NewFormatter("json", ""), false)
	defer logger.Close()

	childA := logger.Slog().With(slog.String("worker", "a"))
	childB := logger.Slog().WithGroup("grp").With(slog.String("worker", "b"))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			childA.Info("msg-a", "n", n)
		}(i)
		go func(n int) {
			defer wg.Done()
			childB.Info("msg-b", "n", n)
		}(i)
	}
	wg.Wait()
	if buf.Len() == 0 {
		t.Fatal("expected log output from concurrent derived loggers")
	}
}

func TestNewConsoleNoColor(t *testing.T) {
	logger, cleanup, err := New(&conf.Log{NoColor: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cleanup()

	cl, ok := logger.(*ConsoleLogger)
	if !ok {
		t.Fatalf("expected *ConsoleLogger, got %T", logger)
	}
	if cl.colorize {
		t.Fatal("expected colorize=false when NoColor is true")
	}
}
