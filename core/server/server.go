package server

import (
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/wire"
)

// ProviderSet is core server providers.
var ProviderSet = wire.NewSet(NewHTTPServer, NewGRPCServer)

// Registrar registers a module's HTTP and gRPC routes onto the shared servers.
// Each business module under internal/<module>/server implements this interface,
// and cmd/server aggregates all module registrars into []Registrar.
type Registrar interface {
	RegisterHTTP(srv *http.Server)
	RegisterGRPC(srv *grpc.Server)
}
