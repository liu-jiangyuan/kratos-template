package service

import (
	"context"

	v1 "github.com/liu-jiangyuan/kratos-template/api/todo/v1"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/biz"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPageSize = 20
	maxPageSize     = 1000
)

// invalidArgument keeps request-parsing failures in the business error channel
// (HTTP 200 with code=0) instead of leaking library errors as 500.
func invalidArgument(err error) error {
	return biz.ErrTodoInvalidArgument.WithCause(err)
}

func normalizePageSize(size int32) int32 {
	if size <= 0 {
		return defaultPageSize
	}
	if size > maxPageSize {
		return maxPageSize
	}
	return size
}

// TodoService implements the generated HTTP handler interface.
type TodoService struct {
	uc *biz.TodoUsecase
}

// NewTodoService creates a new todo service.
func NewTodoService(uc *biz.TodoUsecase) *TodoService {
	return &TodoService{uc: uc}
}

// CreateTodo creates a todo item.
func (s *TodoService) CreateTodo(ctx context.Context, req *v1.CreateTodoRequest) (*v1.Todo, error) {
	todo, err := s.uc.CreateTodo(ctx, convertTodo(req.GetTodo()))
	if err != nil {
		return nil, err
	}
	return convertTodoReply(todo), nil
}

// GetTodo returns a todo item by ID.
func (s *TodoService) GetTodo(ctx context.Context, req *v1.GetTodoRequest) (*v1.Todo, error) {
	id, err := parseTodoID(req.GetId())
	if err != nil {
		return nil, err
	}
	todo, err := s.uc.GetTodo(ctx, id)
	if err != nil {
		return nil, err
	}
	return convertTodoReply(todo), nil
}

// ListTodos lists todo items in creation order.
func (s *TodoService) ListTodos(ctx context.Context, req *v1.ListTodosRequest) (*v1.TodoSet, error) {
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, invalidArgument(err)
	}
	req.PageSize = normalizePageSize(req.PageSize)

	todos, err := s.uc.ListTodos(ctx,
		biz.ListLimit(int(req.PageSize)),
		biz.ListOffset(int(pageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	set := &v1.TodoSet{
		Todos: make([]*v1.Todo, 0, len(todos)),
	}
	if len(todos) >= int(req.PageSize) {
		set.NextPageToken = pageToken.Next(req).String()
	}
	for _, todo := range todos {
		set.Todos = append(set.Todos, convertTodoReply(todo))
	}
	return set, nil
}

// UpdateTodo updates a todo item.
func (s *TodoService) UpdateTodo(ctx context.Context, req *v1.UpdateTodoRequest) (*v1.Todo, error) {
	if req.GetTodo().GetId() == "" || req.GetUpdateMask() == nil || len(req.GetUpdateMask().GetPaths()) == 0 {
		return nil, biz.ErrTodoInvalidArgument
	}
	// The AIP fieldmask helper silently ignores unknown paths; reject them at
	// the service boundary so clients get a clear INVALID_ARGUMENT instead.
	for _, path := range req.GetUpdateMask().GetPaths() {
		switch path {
		case "*", "title", "content", "completed":
		default:
			return nil, biz.ErrTodoInvalidArgument
		}
	}
	current, err := s.GetTodo(ctx, &v1.GetTodoRequest{Id: req.GetTodo().GetId()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.GetUpdateMask(), current, req.GetTodo())
	todo, err := s.uc.UpdateTodo(ctx, convertTodo(current))
	if err != nil {
		return nil, err
	}
	return convertTodoReply(todo), nil
}

// DeleteTodo soft-deletes a todo item.
func (s *TodoService) DeleteTodo(ctx context.Context, req *v1.DeleteTodoRequest) (*emptypb.Empty, error) {
	id, err := parseTodoID(req.GetId())
	if err != nil {
		return nil, err
	}
	if err := s.uc.DeleteTodo(ctx, id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// parseTodoID validates an incoming todo id at the service boundary.
func parseTodoID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, biz.ErrTodoInvalidArgument
	}
	return parsed, nil
}

// convertTodo parses an incoming proto into a DO. Timestamps are omitted: they
// are server-assigned and read-only, so only the mutable fields cross over.
func convertTodo(in *v1.Todo) *biz.Todo {
	if in == nil {
		return nil
	}
	id, _ := uuid.Parse(in.GetId())
	return &biz.Todo{
		ID:        id,
		Title:     in.GetTitle(),
		Content:   in.GetContent(),
		Completed: in.GetCompleted(),
	}
}

func convertTodoReply(in *biz.Todo) *v1.Todo {
	if in == nil {
		return nil
	}
	return &v1.Todo{
		Id:        in.ID.String(),
		Title:     in.Title,
		Content:   in.Content,
		Completed: in.Completed,
		Status:    convertTodoStatus(in.Status),
		CreatedAt: timestamppb.New(in.CreatedAt),
		UpdatedAt: timestamppb.New(in.UpdatedAt),
	}
}

func convertTodoStatus(in biz.TodoStatus) v1.TodoStatus {
	switch in {
	case biz.TodoStatusActive:
		return v1.TodoStatus_TODO_STATUS_ACTIVE
	case biz.TodoStatusDeleted:
		return v1.TodoStatus_TODO_STATUS_DELETED
	default:
		return v1.TodoStatus_TODO_STATUS_UNSPECIFIED
	}
}
