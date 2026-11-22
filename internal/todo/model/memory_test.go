package model

import (
	"context"
	"testing"

	"github.com/liu-jiangyuan/kratos-template/internal/todo/biz"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

func TestMemoryTodoRepoCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryTodoRepo()

	// The demo repo is seeded so `make run` returns data immediately.
	seeded, err := repo.ListTodos(ctx, biz.ListLimit(10))
	if err != nil {
		t.Fatalf("ListTodos(seed) error = %v", err)
	}
	if len(seeded) == 0 {
		t.Fatal("ListTodos(seed) returned no demo data")
	}

	created, err := repo.CreateTodo(ctx, &biz.Todo{Title: "new demo item", Content: "body"})
	if err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("CreateTodo() did not assign an id")
	}
	if created.Status != biz.TodoStatusActive || created.CreatedAt.IsZero() {
		t.Fatalf("CreateTodo() = %+v, want active todo with timestamps", created)
	}

	got, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.Title != "new demo item" {
		t.Fatalf("FindByID() title = %q, want new demo item", got.Title)
	}

	updated, err := repo.UpdateTodo(ctx, &biz.Todo{ID: created.ID, Title: "renamed", Completed: true})
	if err != nil {
		t.Fatalf("UpdateTodo() error = %v", err)
	}
	if updated.Title != "renamed" || !updated.Completed {
		t.Fatalf("UpdateTodo() = %+v, want renamed completed todo", updated)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatal("UpdateTodo() changed created_at")
	}

	if err := repo.DeleteTodo(ctx, created.ID); err != nil {
		t.Fatalf("DeleteTodo() error = %v", err)
	}
	if _, err := repo.FindByID(ctx, created.ID); !kratoserrors.IsNotFound(err) {
		t.Fatalf("FindByID() after delete error = %v, want not found", err)
	}
	if err := repo.DeleteTodo(ctx, created.ID); !kratoserrors.IsNotFound(err) {
		t.Fatalf("DeleteTodo() twice error = %v, want not found", err)
	}
}

func TestMemoryTodoRepoPagination(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryTodoRepo()

	if _, err := repo.CreateTodo(ctx, &biz.Todo{Title: "paged item"}); err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}

	all, err := repo.ListTodos(ctx, biz.ListLimit(1000))
	if err != nil {
		t.Fatalf("ListTodos() error = %v", err)
	}
	if len(all) < 3 {
		t.Fatalf("ListTodos() len = %d, want at least 3 (2 seeded + 1 created)", len(all))
	}

	page, err := repo.ListTodos(ctx, biz.ListLimit(2), biz.ListOffset(1))
	if err != nil {
		t.Fatalf("ListTodos(page) error = %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("ListTodos(page) len = %d, want 2", len(page))
	}
	for i := range page {
		if page[i].ID != all[i+1].ID {
			t.Fatalf("page[%d] id = %v, want %v", i, page[i].ID, all[i+1].ID)
		}
	}

	if _, err := repo.ListTodos(ctx, biz.ListLimit(0)); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("ListTodos(zero limit) error = %v, want bad request", err)
	}
}
