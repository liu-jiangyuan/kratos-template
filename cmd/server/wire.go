//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"log/slog"

	"github.com/liu-jiangyuan/kratos-template/core/conf"
	coreserver "github.com/liu-jiangyuan/kratos-template/core/server"
	"github.com/liu-jiangyuan/kratos-template/internal/todo"

	"github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

// wireApp init kratos application.
//
// The *conf.Data input is unused in the demo. It is kept so enabling the
// database only requires adding data.ProviderSet below and switching the todo
// repository provider to model.NewTodoRepo.
func wireApp(*conf.Server, *conf.Data, *slog.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		coreserver.ProviderSet,
		todo.ProviderSet,
		newRegistrars,
		newApp,
	))
}
