package log

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const defaultHTTPTimeout = 5 * time.Second

// DatabaseConfig configures the HTTP API log database sink (e.g. Elasticsearch, ZincSearch, Loki).
type DatabaseConfig struct {
	// Endpoint is the HTTP URL where formatted log records are sent (e.g. "http://127.0.0.1:4080/api/app_logs/_doc").
	Endpoint string
	// Method is the HTTP method to use (defaults to http.MethodPost).
	Method string
	// Headers holds optional HTTP headers such as Authorization or X-Api-Key.
	Headers map[string]string
	// Timeout is the per-request HTTP timeout (defaults to 5s).
	Timeout time.Duration
	// Client allows injecting a custom *http.Client (optional).
	Client *http.Client
}

// DatabaseLogger ships formatted log records over HTTP to an external log database API.
type DatabaseLogger struct {
	*baseLogger
}

var _ Logger = (*DatabaseLogger)(nil)

type httpLogWriter struct {
	endpoint string
	method   string
	headers  map[string]string
	client   *http.Client
}

func (w *httpLogWriter) Write(p []byte) (int, error) {
	payload := bytes.TrimSpace(p)
	req, err := http.NewRequest(w.method, w.endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("log database HTTP %s %s returned status %d", w.method, w.endpoint, resp.StatusCode)
	}
	return len(p), nil
}

// NewDatabaseLogger creates a Logger that sends each formatted log record via HTTP request
// to an API-driven log database such as Elasticsearch or ZincSearch.
func NewDatabaseLogger(cfg DatabaseConfig, level slog.Level, formatter Formatter, addSource bool) (*DatabaseLogger, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("database log endpoint cannot be empty")
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodPost
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	if formatter == nil {
		formatter = NewFormatter("json", "")
	}
	w := &httpLogWriter{
		endpoint: endpoint,
		method:   method,
		headers:  cfg.Headers,
		client:   client,
	}
	return &DatabaseLogger{
		baseLogger: newBaseLogger(w, nil, level, formatter, addSource),
	}, nil
}
