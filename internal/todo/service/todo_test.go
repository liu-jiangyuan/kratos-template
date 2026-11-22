package service

import (
	"context"
	"testing"
	"time"

	v1 "github.com/liu-jiangyuan/kratos-template/api/todo/v1"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func newTestTodoService() *TodoService {
	uc := biz.NewTodoUsecase(newFakeTodoRepo())
	return NewTodoService(uc)
}

// fakeTodoRepo is an in-memory biz.TodoRepo used to test the service layer in
// isolation from the storage boundary. It preserves insertion order so list
// pagination is deterministic.
type fakeTodoRepo struct {
	order []uuid.UUID
	todos map[uuid.UUID]*biz.Todo
}

func newFakeTodoRepo() biz.TodoRepo {
	return &fakeTodoRepo{todos: make(map[uuid.UUID]*biz.Todo)}
}

func (r *fakeTodoRepo) FindByID(_ context.Context, id uuid.UUID) (*biz.Todo, error) {
	todo, ok := r.todos[id]
	if !ok {
		return nil, biz.ErrTodoNotFound
	}
	return cloneTodo(todo), nil
}

func (r *fakeTodoRepo) ListTodos(_ context.Context, opts ...biz.ListOption) ([]*biz.Todo, error) {
	options := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&options)
	}
	if options.Offset < 0 || options.Limit <= 0 {
		return nil, biz.ErrTodoInvalidArgument
	}

	todos := make([]*biz.Todo, 0, len(r.todos))
	for _, id := range r.order {
		if todo, ok := r.todos[id]; ok {
			todos = append(todos, cloneTodo(todo))
		}
	}

	if options.Offset >= len(todos) {
		return []*biz.Todo{}, nil
	}
	end := min(options.Offset+options.Limit, len(todos))
	return todos[options.Offset:end], nil
}

func (r *fakeTodoRepo) CreateTodo(_ context.Context, todo *biz.Todo) (*biz.Todo, error) {
	now := time.Now()
	todo = cloneTodo(todo)
	todo.ID = uuid.Must(uuid.NewV7())
	todo.Status = biz.TodoStatusActive
	todo.CreatedAt = now
	todo.UpdatedAt = now
	r.order = append(r.order, todo.ID)
	r.todos[todo.ID] = cloneTodo(todo)
	return cloneTodo(todo), nil
}

func (r *fakeTodoRepo) UpdateTodo(_ context.Context, todo *biz.Todo) (*biz.Todo, error) {
	current, ok := r.todos[todo.ID]
	if !ok {
		return nil, biz.ErrTodoNotFound
	}
	updated := cloneTodo(todo)
	// status moves only through DeleteTodo, matching the real repo.
	updated.Status = current.Status
	updated.CreatedAt = current.CreatedAt
	updated.UpdatedAt = time.Now()
	r.todos[updated.ID] = cloneTodo(updated)
	return cloneTodo(updated), nil
}

func (r *fakeTodoRepo) DeleteTodo(_ context.Context, id uuid.UUID) error {
	if _, ok := r.todos[id]; !ok {
		return biz.ErrTodoNotFound
	}
	delete(r.todos, id)
	return nil
}

func cloneTodo(todo *biz.Todo) *biz.Todo {
	if todo == nil {
		return nil
	}
	clone := *todo
	return &clone
}

func TestTodoServiceCRUD(t *testing.T) {
	ctx := context.Background()
	svc := newTestTodoService()

	created, err := svc.CreateTodo(ctx, &v1.CreateTodoRequest{
		Todo: &v1.Todo{
			Title:     "write tests",
			Content:   "cover todo CRUD",
			Completed: false,
		},
	})
	if err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}
	if _, err := uuid.Parse(created.GetId()); err != nil {
		t.Fatalf("CreateTodo() id = %q, want a UUID", created.GetId())
	}
	if created.GetCreatedAt() == nil || created.GetUpdatedAt() == nil {
		t.Fatal("CreateTodo() did not set timestamps")
	}
	if created.GetStatus() != v1.TodoStatus_TODO_STATUS_ACTIVE {
		t.Fatalf("CreateTodo() status = %v, want active", created.GetStatus())
	}

	got, err := svc.GetTodo(ctx, &v1.GetTodoRequest{Id: created.GetId()})
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if got.GetTitle() != "write tests" || got.GetContent() != "cover todo CRUD" {
		t.Fatalf("GetTodo() = %+v, want created todo", got)
	}

	updated, err := svc.UpdateTodo(ctx, &v1.UpdateTodoRequest{
		Todo: &v1.Todo{
			Id:        created.GetId(),
			Title:     "write service tests",
			Completed: true,
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title", "completed"}},
	})
	if err != nil {
		t.Fatalf("UpdateTodo() error = %v", err)
	}
	if updated.GetTitle() != "write service tests" || !updated.GetCompleted() {
		t.Fatalf("UpdateTodo() = %+v, want updated title and completed", updated)
	}
	if updated.GetContent() != "cover todo CRUD" {
		t.Fatalf("UpdateTodo() content = %q, want original content", updated.GetContent())
	}
	// status is read-only: an update cannot move it, only DeleteTodo can.
	if updated.GetStatus() != v1.TodoStatus_TODO_STATUS_ACTIVE {
		t.Fatalf("UpdateTodo() status = %v, want active", updated.GetStatus())
	}

	if _, err := svc.DeleteTodo(ctx, &v1.DeleteTodoRequest{Id: created.GetId()}); err != nil {
		t.Fatalf("DeleteTodo() error = %v", err)
	}
	if _, err := svc.GetTodo(ctx, &v1.GetTodoRequest{Id: created.GetId()}); !kratoserrors.IsNotFound(err) {
		t.Fatalf("GetTodo() after delete error = %v, want not found", err)
	}
}

func TestTodoServiceListTodosPagination(t *testing.T) {
	ctx := context.Background()
	svc := newTestTodoService()

	for _, title := range []string{"first", "second", "third"} {
		if _, err := svc.CreateTodo(ctx, &v1.CreateTodoRequest{Todo: &v1.Todo{Title: title}}); err != nil {
			t.Fatalf("CreateTodo(%q) error = %v", title, err)
		}
	}

	firstPage, err := svc.ListTodos(ctx, &v1.ListTodosRequest{PageSize: 2})
	if err != nil {
		t.Fatalf("ListTodos(first page) error = %v", err)
	}
	if len(firstPage.GetTodos()) != 2 {
		t.Fatalf("ListTodos(first page) len = %d, want 2", len(firstPage.GetTodos()))
	}
	if firstPage.GetNextPageToken() == "" {
		t.Fatal("ListTodos(first page) next_page_token is empty")
	}

	secondPage, err := svc.ListTodos(ctx, &v1.ListTodosRequest{
		PageSize:  2,
		PageToken: firstPage.GetNextPageToken(),
	})
	if err != nil {
		t.Fatalf("ListTodos(second page) error = %v", err)
	}
	if len(secondPage.GetTodos()) != 1 {
		t.Fatalf("ListTodos(second page) len = %d, want 1", len(secondPage.GetTodos()))
	}
	if secondPage.GetNextPageToken() != "" {
		t.Fatalf("ListTodos(second page) next_page_token = %q, want empty", secondPage.GetNextPageToken())
	}
	if secondPage.GetTodos()[0].GetTitle() != "third" {
		t.Fatalf("ListTodos(second page) title = %q, want third", secondPage.GetTodos()[0].GetTitle())
	}
}

func TestTodoServiceValidation(t *testing.T) {
	ctx := context.Background()
	svc := newTestTodoService()
	missingID := uuid.Must(uuid.NewV7()).String()

	if _, err := svc.CreateTodo(ctx, &v1.CreateTodoRequest{Todo: &v1.Todo{Title: " "}}); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("CreateTodo(empty title) error = %v, want bad request", err)
	}
	if _, err := svc.GetTodo(ctx, &v1.GetTodoRequest{Id: "not-a-uuid"}); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("GetTodo(malformed id) error = %v, want bad request", err)
	}
	if _, err := svc.DeleteTodo(ctx, &v1.DeleteTodoRequest{Id: ""}); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("DeleteTodo(empty id) error = %v, want bad request", err)
	}
	if _, err := svc.UpdateTodo(ctx, &v1.UpdateTodoRequest{
		Todo:       &v1.Todo{Id: missingID, Title: "missing mask"},
		UpdateMask: &fieldmaskpb.FieldMask{},
	}); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("UpdateTodo(empty mask) error = %v, want bad request", err)
	}
	if _, err := svc.ListTodos(ctx, &v1.ListTodosRequest{PageToken: "bad-token"}); err == nil {
		t.Fatal("ListTodos(bad token) error = nil, want error")
	}
	if _, err := svc.DeleteTodo(ctx, &v1.DeleteTodoRequest{Id: missingID}); !kratoserrors.IsNotFound(err) {
		t.Fatalf("DeleteTodo(missing id) error = %v, want not found", err)
	}
}
