package server

import (
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu-jiangyuan/kratos-template/pkg/response"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) response.Response {
	t.Helper()
	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v, body = %q", err, rec.Body.String())
	}
	return got
}

func TestResponseEncoder_Success(t *testing.T) {
	req := httptest.NewRequest(stdhttp.MethodGet, "/v1/todos/1", nil)
	rec := httptest.NewRecorder()

	data := map[string]any{"id": "1", "title": "buy milk"}
	if err := ResponseEncoder(rec, req, data); err != nil {
		t.Fatalf("ResponseEncoder() error = %v", err)
	}

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != jsonContentType {
		t.Fatalf("Content-Type = %q, want %q", ct, jsonContentType)
	}

	got := decodeEnvelope(t, rec)
	if got.Code != response.CodeSuccess || got.Message != response.DefaultSuccessMsg {
		t.Fatalf("unexpected envelope: %+v", got)
	}
}

// TestResponseEncoder_IgnoresProtojsonAccept guards against the generated HTTP
// client's default "Accept: application/protojson" causing a marshal error:
// the envelope is not a proto.Message, so the encoder must always use JSON.
func TestResponseEncoder_IgnoresProtojsonAccept(t *testing.T) {
	req := httptest.NewRequest(stdhttp.MethodGet, "/v1/todos/1", nil)
	req.Header.Set("Accept", "application/protojson")
	rec := httptest.NewRecorder()

	if err := ResponseEncoder(rec, req, map[string]any{"id": "1"}); err != nil {
		t.Fatalf("ResponseEncoder() error = %v", err)
	}
	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", rec.Code, stdhttp.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != jsonContentType {
		t.Fatalf("Content-Type = %q, want %q", ct, jsonContentType)
	}
}

func TestErrorEncoder_StatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "validation failure stays 200",
			err:        kratoserrors.BadRequest("VALIDATOR", "title is required"),
			wantStatus: stdhttp.StatusOK,
			wantMsg:    "title is required",
		},
		{
			name:       "business not found stays 200",
			err:        kratoserrors.NotFound("TODO_NOT_FOUND", "todo not found"),
			wantStatus: stdhttp.StatusOK,
			wantMsg:    "todo not found",
		},
		{
			name:       "conflict stays 200",
			err:        kratoserrors.Conflict("TODO_CONFLICT", "already exists"),
			wantStatus: stdhttp.StatusOK,
			wantMsg:    "already exists",
		},
		{
			name:       "malformed request body keeps 400",
			err:        kratoserrors.BadRequest("CODEC", "body unmarshal failed"),
			wantStatus: stdhttp.StatusBadRequest,
			wantMsg:    "body unmarshal failed",
		},
		{
			name:       "unauthorized keeps 401",
			err:        kratoserrors.Unauthorized("NO_TOKEN", "token missing"),
			wantStatus: stdhttp.StatusUnauthorized,
			wantMsg:    "token missing",
		},
		{
			name:       "forbidden keeps 403",
			err:        kratoserrors.Forbidden("NO_PERM", "permission denied"),
			wantStatus: stdhttp.StatusForbidden,
			wantMsg:    "permission denied",
		},
		{
			name:       "internal keeps 500 and hides details",
			err:        kratoserrors.InternalServer("TODO_INTERNAL", "pq: relation todos does not exist"),
			wantStatus: stdhttp.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
		{
			name:       "bad gateway keeps 502",
			err:        kratoserrors.New(stdhttp.StatusBadGateway, "UPSTREAM", "upstream failed"),
			wantStatus: stdhttp.StatusBadGateway,
			wantMsg:    "bad gateway",
		},
		{
			name:       "unavailable keeps 503",
			err:        kratoserrors.ServiceUnavailable("DB_DOWN", "db down"),
			wantStatus: stdhttp.StatusServiceUnavailable,
			wantMsg:    "service unavailable",
		},
		{
			name:       "timeout keeps 504",
			err:        kratoserrors.GatewayTimeout("TIMEOUT", "timeout"),
			wantStatus: stdhttp.StatusGatewayTimeout,
			wantMsg:    "gateway timeout",
		},
		{
			name:       "raw error becomes 500",
			err:        fmt.Errorf("sql: connection refused"),
			wantStatus: stdhttp.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(stdhttp.MethodGet, "/v1/todos/1", nil)
			rec := httptest.NewRecorder()

			ErrorEncoder(rec, req, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("HTTP status = %d, want %d", rec.Code, tt.wantStatus)
			}
			got := decodeEnvelope(t, rec)
			if got.Code != response.CodeFail {
				t.Fatalf("envelope code = %d, want %d", got.Code, response.CodeFail)
			}
			if got.Message != tt.wantMsg {
				t.Fatalf("envelope message = %q, want %q", got.Message, tt.wantMsg)
			}
		})
	}
}

func TestRouterErrorHandlers(t *testing.T) {
	t.Run("404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		EnvelopeNotFoundHandler().ServeHTTP(rec, httptest.NewRequest(stdhttp.MethodGet, "/missing", nil))
		if rec.Code != stdhttp.StatusNotFound {
			t.Fatalf("HTTP status = %d, want %d", rec.Code, stdhttp.StatusNotFound)
		}
		got := decodeEnvelope(t, rec)
		if got.Code != response.CodeFail || got.Message != "not found" {
			t.Fatalf("unexpected envelope: %+v", got)
		}
	})

	t.Run("405", func(t *testing.T) {
		rec := httptest.NewRecorder()
		EnvelopeMethodNotAllowedHandler().ServeHTTP(rec, httptest.NewRequest(stdhttp.MethodPost, "/v1/todos/1", nil))
		if rec.Code != stdhttp.StatusMethodNotAllowed {
			t.Fatalf("HTTP status = %d, want %d", rec.Code, stdhttp.StatusMethodNotAllowed)
		}
		got := decodeEnvelope(t, rec)
		if got.Code != response.CodeFail || got.Message != "method not allowed" {
			t.Fatalf("unexpected envelope: %+v", got)
		}
	})
}
