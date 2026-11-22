package model

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/liu-jiangyuan/kratos-template/internal/todo/biz"

	"github.com/google/uuid"
)

// NewMemoryTodoRepo returns an in-memory biz.TodoRepo seeded with a couple of
// demo items. It is the default repository so `make run` needs no database.
// Swap it for NewTodoRepo and add core/data.ProviderSet to enable Ent/Postgres.
func NewMemoryTodoRepo() biz.TodoRepo {
	r := &memoryTodoRepo{todos: make(map[uuid.UUID]*biz.Todo)}
	r.seed("Try the todo API", "Create, list, update, and delete items over HTTP.")
	r.seed("Read the architecture docs", "Start with docs/README.md.")
	return r
}

type memoryTodoRepo struct {
	mu    sync.RWMutex
	todos map[uuid.UUID]*biz.Todo
}

func (r *memoryTodoRepo) seed(title, content string) {
	now := time.Now()
	id := uuid.Must(uuid.NewV7())
	r.todos[id] = &biz.Todo{
		ID:        id,
		Title:     title,
		Content:   content,
		Status:    biz.TodoStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (r *memoryTodoRepo) FindByID(_ context.Context, id uuid.UUID) (*biz.Todo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	todo, ok := r.todos[id]
	if !ok || todo.Status != biz.TodoStatusActive {
		return nil, biz.ErrTodoNotFound
	}
	return cloneTodo(todo), nil
}

func (r *memoryTodoRepo) ListTodos(_ context.Context, opts ...biz.ListOption) ([]*biz.Todo, error) {
	options := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&options)
	}
	if options.Offset < 0 || options.Limit <= 0 {
		return nil, biz.ErrTodoInvalidArgument
	}

	r.mu.RLock()
	todos := make([]*biz.Todo, 0, len(r.todos))
	for _, todo := range r.todos {
		if todo.Status == biz.TodoStatusActive {
			todos = append(todos, cloneTodo(todo))
		}
	}
	r.mu.RUnlock()

	sort.Slice(todos, func(i, j int) bool {
		if todos[i].CreatedAt.Equal(todos[j].CreatedAt) {
			return todos[i].ID.String() < todos[j].ID.String()
		}
		return todos[i].CreatedAt.Before(todos[j].CreatedAt)
	})
	if options.Offset >= len(todos) {
		return []*biz.Todo{}, nil
	}
	end := min(options.Offset+options.Limit, len(todos))
	return todos[options.Offset:end], nil
}

func (r *memoryTodoRepo) CreateTodo(_ context.Context, todo *biz.Todo) (*biz.Todo, error) {
	now := time.Now()
	created := cloneTodo(todo)
	created.ID = uuid.Must(uuid.NewV7())
	created.Status = biz.TodoStatusActive
	created.CreatedAt = now
	created.UpdatedAt = now

	r.mu.Lock()
	r.todos[created.ID] = cloneTodo(created)
	r.mu.Unlock()
	return cloneTodo(created), nil
}

func (r *memoryTodoRepo) UpdateTodo(_ context.Context, todo *biz.Todo) (*biz.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.todos[todo.ID]
	if !ok || current.Status != biz.TodoStatusActive {
		return nil, biz.ErrTodoNotFound
	}
	updated := cloneTodo(todo)
	updated.Status = current.Status
	updated.CreatedAt = current.CreatedAt
	updated.UpdatedAt = time.Now()
	r.todos[updated.ID] = cloneTodo(updated)
	return cloneTodo(updated), nil
}

func (r *memoryTodoRepo) DeleteTodo(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	todo, ok := r.todos[id]
	if !ok || todo.Status != biz.TodoStatusActive {
		return biz.ErrTodoNotFound
	}
	todo.Status = biz.TodoStatusDeleted
	return nil
}

func cloneTodo(todo *biz.Todo) *biz.Todo {
	if todo == nil {
		return nil
	}
	cp := *todo
	return &cp
}
