package server

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/liu-jiangyuan/kratos-template/pkg/response"

	"github.com/go-kratos/kratos/v3/errors"
)

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

	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if got.Code != response.CodeSuccess || got.Msg != response.DefaultSuccessMsg {
		t.Fatalf("unexpected envelope: %+v", got)
	}
}

func TestErrorEncoder_BusinessErrorKeepsHTTP200(t *testing.T) {
	req := httptest.NewRequest(stdhttp.MethodGet, "/v1/todos/invalid", nil)
	rec := httptest.NewRecorder()

	bizErr := errors.BadRequest("TODO_INVALID_ARGUMENT", "invalid todo argument")
	ErrorEncoder(rec, req, bizErr)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("HTTP status = %d, want %d (200 OK)", rec.Code, stdhttp.StatusOK)
	}

	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if got.Code != response.CodeFail || got.Msg != "invalid todo argument" || got.Data != nil {
		t.Fatalf("unexpected error envelope: %+v", got)
	}
}
