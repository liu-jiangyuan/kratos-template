//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"log/slog"

	"github.com/liu-jiangyuan/kratos-template/core/conf"
	"github.com/liu-jiangyuan/kratos-template/core/data"
	coreserver "github.com/liu-jiangyuan/kratos-template/core/server"
	"github.com/liu-jiangyuan/kratos-template/internal/todo"


	"github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, *slog.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		coreserver.ProviderSet,
		data.ProviderSet,
		todo.ProviderSet,
		newRegistrars,
		newApp,
	))
}

