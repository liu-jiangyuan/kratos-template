package requestid

import (
	"context"
	"log/slog"
	"strings"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/uuid"
)

const (
	// DefaultHeader is the default HTTP/gRPC header name used for request IDs.
	DefaultHeader = "X-Request-Id"
	// LogKey is the slog attribute key injected into log records.
	LogKey = "request_id"
)

type ctxKey struct{}

// Option configures the request ID middleware.
type Option func(*options)

type options struct {
	header    string
	generator func() string
}

// WithHeader sets a custom header name for extracting and propagating the request ID.
func WithHeader(header string) Option {
	return func(o *options) {
		if h := strings.TrimSpace(header); h != "" {
			o.header = h
		}
	}
}

// WithGenerator sets a custom ID generator function when the incoming request
// does not carry a request ID header.
func WithGenerator(fn func() string) Option {
	return func(o *options) {
		if fn != nil {
			o.generator = fn
		}
	}
}

func defaultGenerator() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	return uuid.NewString()
}

// Server returns a Kratos server middleware that extracts the request ID from
// the incoming request header, or generates a new one if absent. It writes the
// request ID back to the reply header, stores it in context, and attaches it
// to Kratos log context attributes so every context-aware log entry includes it.
func Server(opts ...Option) middleware.Middleware {
	cfg := &options{
		header:    DefaultHeader,
		generator: defaultGenerator,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			var reqID string
			if tr, ok := transport.FromServerContext(ctx); ok {
				if header := tr.RequestHeader(); header != nil {
					reqID = strings.TrimSpace(header.Get(cfg.header))
				}
				if reqID == "" {
					reqID = cfg.generator()
				}
				if replyHeader := tr.ReplyHeader(); replyHeader != nil {
					replyHeader.Set(cfg.header, reqID)
				}
			} else {
				reqID = FromContext(ctx)
				if reqID == "" {
					reqID = cfg.generator()
				}
			}

			ctx = NewContext(ctx, reqID)
			ctx = log.ContextWithAttrs(ctx, slog.String(LogKey, reqID))
			return handler(ctx, req)
		}
	}
}

// Client returns a Kratos client middleware that propagates the request ID
// from context into outgoing client request headers.
func Client(opts ...Option) middleware.Middleware {
	cfg := &options{
		header:    DefaultHeader,
		generator: defaultGenerator,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			reqID := FromContext(ctx)
			if reqID == "" {
				reqID = cfg.generator()
				ctx = NewContext(ctx, reqID)
			}
			if tr, ok := transport.FromClientContext(ctx); ok {
				if header := tr.RequestHeader(); header != nil && header.Get(cfg.header) == "" {
					header.Set(cfg.header, reqID)
				}
			}
			return handler(ctx, req)
		}
	}
}

// NewContext returns a new context carrying the given request ID.
func NewContext(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, reqID)
}

// FromContext extracts the request ID from ctx, returning an empty string if absent.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}
