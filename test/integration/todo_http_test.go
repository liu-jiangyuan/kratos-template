package integration_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/liu-jiangyuan/kratos-template/api/todo/v1"
	"github.com/liu-jiangyuan/kratos-template/core/conf"
	coreserver "github.com/liu-jiangyuan/kratos-template/core/server"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/biz"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/model"
	todoserver "github.com/liu-jiangyuan/kratos-template/internal/todo/server"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/service"
	"github.com/liu-jiangyuan/kratos-template/pkg/response"

	"github.com/google/uuid"
)

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	repo := model.NewMemoryTodoRepo()
	svc := service.NewTodoService(biz.NewTodoUsecase(repo))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := coreserver.NewHTTPServer(&conf.Server{}, logger, []coreserver.Registrar{todoserver.NewRegistrar(svc)})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, method, url, body string, out any) (int, envelope) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("%s %s: decode envelope %q: %v", method, url, raw, err)
	}
	if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, out); err != nil {
			t.Fatalf("decode data: %v", err)
		}
	}
	return resp.StatusCode, env
}

func TestTodoHTTPContract(t *testing.T) {
	ts := newTestServer(t)

	var created v1.Todo
	status, env := doJSON(t, http.MethodPost, ts.URL+"/v1/todos", `{"title":"first","content":"body"}`, &created)
	if status != http.StatusOK || env.Code != response.CodeSuccess {
		t.Fatalf("create: status=%d code=%d message=%q", status, env.Code, env.Message)
	}
	if created.Id == "" || created.Title != "first" {
		t.Fatalf("create returned id=%q title=%q content=%q", created.Id, created.Title, created.Content)
	}

	var got v1.Todo
	status, env = doJSON(t, http.MethodGet, ts.URL+"/v1/todos/"+created.Id, "", &got)
	if status != http.StatusOK || env.Code != response.CodeSuccess || got.Id != created.Id {
		t.Fatalf("get: status=%d code=%d id=%q", status, env.Code, got.Id)
	}

	var updated v1.Todo
	// update_mask is a FieldMask query parameter; content must stay unchanged.
	status, env = doJSON(t, http.MethodPatch,
		ts.URL+"/v1/todos/"+created.Id+"?update_mask=title",
		`{"title":"renamed","content":"ignored"}`,
		&updated)
	if status != http.StatusOK || env.Code != response.CodeSuccess {
		t.Fatalf("update: status=%d code=%d message=%q", status, env.Code, env.Message)
	}
	if updated.Title != "renamed" || updated.Content != "body" {
		t.Fatalf("update mask not applied: title=%q content=%q", updated.Title, updated.Content)
	}

	// Business not-found stays HTTP 200 with code=0.
	status, env = doJSON(t, http.MethodGet, ts.URL+"/v1/todos/"+uuid.Must(uuid.NewV7()).String(), "", nil)
	if status != http.StatusOK || env.Code != response.CodeFail {
		t.Fatalf("business not found: status=%d code=%d message=%q", status, env.Code, env.Message)
	}

	// Malformed request body is a protocol error and keeps 400.
	status, env = doJSON(t, http.MethodPost, ts.URL+"/v1/todos", "{", nil)
	if status != http.StatusBadRequest || env.Code != response.CodeFail {
		t.Fatalf("malformed body: status=%d code=%d message=%q", status, env.Code, env.Message)
	}
}
