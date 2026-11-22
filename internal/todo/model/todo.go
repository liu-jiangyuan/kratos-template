package model

import (
	"context"

	"github.com/liu-jiangyuan/kratos-template/core/data"
	"github.com/liu-jiangyuan/kratos-template/ent"
	"github.com/liu-jiangyuan/kratos-template/ent/todo"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/biz"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"
)

// internalError logs the storage failure (keeping request_id/trace_id from ctx)
// and returns the client-safe domain error.
func internalError(ctx context.Context, err error) error {
	log.ErrorContext(ctx, "todo repository error", "err", err)
	return biz.ErrTodoInternal
}

// toBiz converts a persisted todo into its domain representation.
func toBiz(po *ent.Todo) *biz.Todo {
	if po == nil {
		return nil
	}
	return &biz.Todo{
		ID:        po.ID,
		Title:     po.Title,
		Content:   po.Content,
		Completed: po.Completed,
		Status:    biz.TodoStatus(po.Status),
		CreatedAt: po.CreatedAt,
		UpdatedAt: po.UpdatedAt,
	}
}

type todoRepo struct {
	data *data.Data
}

// NewTodoRepo creates a new TodoRepo instance.
func NewTodoRepo(d *data.Data) biz.TodoRepo {
	return &todoRepo{data: d}
}

func (r *todoRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.Todo, error) {
	po, err := r.data.DB(ctx).Todo.Query().
		Where(todo.IDEQ(id), todo.StatusEQ(int32(biz.TodoStatusActive))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrTodoNotFound
		}
		return nil, internalError(ctx, err)
	}
	return toBiz(po), nil
}

func (r *todoRepo) ListTodos(ctx context.Context, opts ...biz.ListOption) ([]*biz.Todo, error) {
	options := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&options)
	}
	if options.Offset < 0 || options.Limit <= 0 {
		return nil, biz.ErrTodoInvalidArgument
	}
	// UUIDv7 ids are time-ordered, so ordering by id gives a stable creation
	// order and makes offset pagination deterministic.
	pos, err := r.data.DB(ctx).Todo.Query().
		Where(todo.StatusEQ(int32(biz.TodoStatusActive))).
		Order(todo.ByID()).
		Offset(options.Offset).
		Limit(options.Limit).
		All(ctx)
	if err != nil {
		return nil, internalError(ctx, err)
	}
	todos := make([]*biz.Todo, 0, len(pos))
	for _, po := range pos {
		todos = append(todos, toBiz(po))
	}
	return todos, nil
}

func (r *todoRepo) CreateTodo(ctx context.Context, t *biz.Todo) (*biz.Todo, error) {
	po, err := r.data.DB(ctx).Todo.Create().
		SetTitle(t.Title).
		SetContent(t.Content).
		SetCompleted(t.Completed).
		SetStatus(int32(biz.TodoStatusActive)).
		Save(ctx)
	if err != nil {
		return nil, internalError(ctx, err)
	}
	return toBiz(po), nil
}

func (r *todoRepo) UpdateTodo(ctx context.Context, t *biz.Todo) (*biz.Todo, error) {
	// status is not updatable here: it moves only through DeleteTodo.
	po, err := r.data.DB(ctx).Todo.UpdateOneID(t.ID).
		Where(todo.StatusEQ(int32(biz.TodoStatusActive))).
		SetTitle(t.Title).
		SetContent(t.Content).
		SetCompleted(t.Completed).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrTodoNotFound
		}
		return nil, internalError(ctx, err)
	}
	return toBiz(po), nil
}

// DeleteTodo soft-deletes a todo by flipping its status to deleted. The row is
// kept so it stays auditable, and every read path filters it out.
func (r *todoRepo) DeleteTodo(ctx context.Context, id uuid.UUID) error {
	affected, err := r.data.DB(ctx).Todo.Update().
		Where(todo.IDEQ(id), todo.StatusEQ(int32(biz.TodoStatusActive))).
		SetStatus(int32(biz.TodoStatusDeleted)).
		Save(ctx)
	if err != nil {
		return internalError(ctx, err)
	}
	if affected == 0 {
		return biz.ErrTodoNotFound
	}
	return nil
}
