package server

import (
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/wire"
)

// ProviderSet is core server providers.
var ProviderSet = wire.NewSet(NewHTTPServer)

// Registrar registers a module's HTTP routes onto the shared server. Each
// business module under internal/<module>/server implements this interface,
// and cmd/server aggregates all module registrars into []Registrar.
type Registrar interface {
	RegisterHTTP(srv *http.Server)
}
