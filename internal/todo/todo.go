package todo

import (
	"github.com/liu-jiangyuan/kratos-template/internal/todo/biz"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/model"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/server"
	"github.com/liu-jiangyuan/kratos-template/internal/todo/service"

	"github.com/google/wire"
)

// ProviderSet is the wire provider set for the todo module.
//
// The demo uses the in-memory repository so `make run` needs no database. To
// switch to Ent/Postgres, replace model.NewMemoryTodoRepo with model.NewTodoRepo
// and add core/data.ProviderSet to cmd/server/wire.go.
var ProviderSet = wire.NewSet(
	model.NewMemoryTodoRepo,
	biz.NewTodoUsecase,
	service.NewTodoService,
	server.NewRegistrar,
)
