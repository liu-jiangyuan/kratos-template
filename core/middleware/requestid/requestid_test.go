package requestid

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport"
)

type fakeHeader map[string][]string

func (h fakeHeader) Get(key string) string {
	vals := h[strings.ToLower(key)]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (h fakeHeader) Set(key, value string) {
	h[strings.ToLower(key)] = []string{value}
}

func (h fakeHeader) Add(key, value string) {
	k := strings.ToLower(key)
	h[k] = append(h[k], value)
}

func (h fakeHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}

func (h fakeHeader) Values(key string) []string {
	return h[strings.ToLower(key)]
}

type fakeTransporter struct {
	reqHeader   fakeHeader
	replyHeader fakeHeader
}

func (t *fakeTransporter) Kind() transport.Kind            { return transport.KindHTTP }
func (t *fakeTransporter) Endpoint() string                { return "http://127.0.0.1:8000" }
func (t *fakeTransporter) Operation() string               { return "/todo.v1.TodoService/GetTodo" }
func (t *fakeTransporter) RequestHeader() transport.Header { return t.reqHeader }
func (t *fakeTransporter) ReplyHeader() transport.Header   { return t.replyHeader }

func TestServerGeneratesRequestIDWhenMissing(t *testing.T) {
	tr := &fakeTransporter{
		reqHeader:   fakeHeader{},
		replyHeader: fakeHeader{},
	}
	ctx := transport.NewServerContext(context.Background(), tr)

	var buf bytes.Buffer
	logger := log.NewLogger(slog.NewJSONHandler(&buf, nil))

	mw := Server()
	var capturedID string
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		capturedID = FromContext(ctx)
		logger.InfoContext(ctx, "inside handler")
		return "ok", nil
	})(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID == "" {
		t.Fatal("expected generated request ID in context, got empty")
	}
	if got := tr.replyHeader.Get(DefaultHeader); got != capturedID {
		t.Fatalf("reply header %s = %q, want %q", DefaultHeader, got, capturedID)
	}
	if !strings.Contains(buf.String(), `"request_id":"`+capturedID+`"`) {
		t.Fatalf("log output %q does not contain request_id %q", buf.String(), capturedID)
	}
}

func TestServerPreservesIncomingRequestID(t *testing.T) {
	const incomingID = "req-custom-12345"
	tr := &fakeTransporter{
		reqHeader:   fakeHeader{strings.ToLower(DefaultHeader): []string{incomingID}},
		replyHeader: fakeHeader{},
	}
	ctx := transport.NewServerContext(context.Background(), tr)

	mw := Server()
	var capturedID string
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		capturedID = FromContext(ctx)
		return "ok", nil
	})(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != incomingID {
		t.Fatalf("FromContext() = %q, want %q", capturedID, incomingID)
	}
	if got := tr.replyHeader.Get(DefaultHeader); got != incomingID {
		t.Fatalf("reply header = %q, want %q", got, incomingID)
	}
}
