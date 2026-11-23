package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu-jiangyuan/kratos-template/core/conf"
	"github.com/liu-jiangyuan/kratos-template/pkg/response"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

// contractRegistrar registers a few synthetic routes so the HTTP status
// contract can be exercised through the real middleware chain and router.
type contractRegistrar struct{}

func (contractRegistrar) RegisterHTTP(s *kratoshttp.Server) {
	r := s.Route("/")
	r.GET("/ok", func(ctx kratoshttp.Context) error {
		return ctx.Result(http.StatusOK, map[string]string{"hello": "world"})
	})
	r.GET("/business", func(kratoshttp.Context) error {
		return kratoserrors.NotFound("TODO_NOT_FOUND", "todo not found")
	})
	r.GET("/protocol", func(kratoshttp.Context) error {
		return kratoserrors.BadRequest("CODEC", "bad request body")
	})
	r.GET("/internal", func(kratoshttp.Context) error {
		return kratoserrors.InternalServer("INTERNAL", "pq: relation todos does not exist")
	})
}

func newContractServer(t *testing.T) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewHTTPServer(&conf.Server{}, logger, []Registrar{contractRegistrar{}})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, ts *httptest.Server, method, path, accept string) (*http.Response, response.Response) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var envelope response.Response
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("%s %s: decode body: %v", method, path, err)
	}
	return resp, envelope
}

func TestHTTPStatusContract(t *testing.T) {
	ts := newContractServer(t)

	tests := []struct {
		name       string
		method     string
		path       string
		accept     string
		wantStatus int
		wantCode   int
		wantMsg    string
	}{
		{
			name:       "healthz returns 200 code 1",
			method:     http.MethodGet,
			path:       "/healthz",
			wantStatus: http.StatusOK,
			wantCode:   response.CodeSuccess,
		},
		{
			name:       "success is 200 code 1",
			method:     http.MethodGet,
			path:       "/ok",
			wantStatus: http.StatusOK,
			wantCode:   response.CodeSuccess,
		},
		{
			name:       "protojson accept still returns JSON envelope",
			method:     http.MethodGet,
			path:       "/ok",
			accept:     "application/protojson",
			wantStatus: http.StatusOK,
			wantCode:   response.CodeSuccess,
		},
		{
			name:       "business error is 200 code 0",
			method:     http.MethodGet,
			path:       "/business",
			wantStatus: http.StatusOK,
			wantCode:   response.CodeFail,
			wantMsg:    "todo not found",
		},
		{
			name:       "protocol error keeps 400",
			method:     http.MethodGet,
			path:       "/protocol",
			wantStatus: http.StatusBadRequest,
			wantCode:   response.CodeFail,
			wantMsg:    "bad request body",
		},
		{
			name:       "internal error keeps 500 and hides details",
			method:     http.MethodGet,
			path:       "/internal",
			wantStatus: http.StatusInternalServerError,
			wantCode:   response.CodeFail,
			wantMsg:    "internal server error",
		},
		{
			name:       "unknown route keeps 404",
			method:     http.MethodGet,
			path:       "/missing",
			wantStatus: http.StatusNotFound,
			wantCode:   response.CodeFail,
			wantMsg:    "not found",
		},
		{
			name:       "wrong method keeps 405",
			method:     http.MethodPost,
			path:       "/ok",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   response.CodeFail,
			wantMsg:    "method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, envelope := doJSON(t, ts, tt.method, tt.path, tt.accept)
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("HTTP status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if ct := resp.Header.Get("Content-Type"); ct != jsonContentType {
				t.Fatalf("Content-Type = %q, want %q", ct, jsonContentType)
			}
			if envelope.Code != tt.wantCode {
				t.Fatalf("envelope code = %d, want %d", envelope.Code, tt.wantCode)
			}
			if tt.wantMsg != "" && envelope.Message != tt.wantMsg {
				t.Fatalf("envelope message = %q, want %q", envelope.Message, tt.wantMsg)
			}
		})
	}
}
