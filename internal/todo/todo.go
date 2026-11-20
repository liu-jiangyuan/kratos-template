package todo

import (
	"github.com/liu-jiangyuan/kratos-template/internal/todo/biz"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/model"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/server"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/service"

	"github.com/google/wire"
)

// ProviderSet is the wire provider set for the todo module.
var ProviderSet = wire.NewSet(
	model.NewTodoRepo,
	biz.NewTodoUsecase,
	service.NewTodoService,
	server.NewRegistrar,
)
