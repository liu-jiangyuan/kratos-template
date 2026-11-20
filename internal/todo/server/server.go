package server

import (
	v1 "github.com/liu-jiangyuan/kratos-template/api/todo/v1"
	coreserver "github.com/liu-jiangyuan/kratos-template/core/server"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/service"

	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"
)

// Registrar registers the todo module's HTTP and gRPC routes onto the shared servers.
type Registrar struct {
	svc *service.TodoService
}

var _ coreserver.Registrar = (*Registrar)(nil)

// NewRegistrar creates a new route registrar for the todo module.
func NewRegistrar(svc *service.TodoService) *Registrar {
	return &Registrar{svc: svc}
}

// RegisterHTTP registers the todo module's HTTP routes.
func (r *Registrar) RegisterHTTP(srv *http.Server) {
	v1.RegisterTodoServiceHTTPServer(srv, r.svc)
}

// RegisterGRPC registers the todo module's gRPC service.
func (r *Registrar) RegisterGRPC(srv *grpc.Server) {
	v1.RegisterTodoServiceServer(srv, r.svc)
}
