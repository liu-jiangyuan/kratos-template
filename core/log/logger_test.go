package log

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
}

func TestDatabaseLoggerHTTP(t *testing.T) {
	var (
		mu       sync.Mutex
		received []map[string]any
		authHdr  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		authHdr = r.Header.Get("Authorization")
		received = append(received, doc)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	logger, err := NewDatabaseLogger(DatabaseConfig{
		Endpoint: srv.URL + "/api/logs/_doc",
		Method:   http.MethodPost,
		Headers: map[string]string{
			"Authorization": "Basic dGVzdDp0ZXN0",
		},
	}, slog.LevelInfo, NewFormatter("json", ""), false)
	if err != nil {
		t.Fatalf("NewDatabaseLogger() error = %v", err)
	}
	defer logger.Close()

	ctx := kratoslog.ContextWithAttrs(context.Background(), slog.String("request_id", "req-db-777"))
	logger.Warn(ctx, "slow query detected", "cost_ms", 320)

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
