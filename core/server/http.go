package server

import (
	"log/slog"

	"github.com/liu-jiangyuan/kratos-template/core/conf"
	"github.com/liu-jiangyuan/kratos-template/core/middleware/requestid"

	"github.com/go-kratos/kratos/v3/middleware/logging"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/http"
	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// NewHTTPServer creates the shared HTTP server and registers all module routes.
func NewHTTPServer(c *conf.Server, logger *slog.Logger, registrars []Registrar) *http.Server {
	var opts = []http.ServerOption{
		http.ResponseEncoder(ResponseEncoder),
		http.ErrorEncoder(ErrorEncoder),
		http.NotFoundHandler(EnvelopeNotFoundHandler()),
		http.MethodNotAllowedHandler(EnvelopeMethodNotAllowedHandler()),
		http.Middleware(
			recovery.Recovery(),
			requestid.Server(),
			logging.Server(logger),
			validate.Validator(func(req any) error {
				if msg, ok := req.(proto.Message); ok {
					if err := fieldbehavior.ValidateRequiredFields(msg); err != nil {
						return err
					}
				}
				return nil
			}),
		),
	}
	if c.GetHttp().GetNetwork() != "" {
		opts = append(opts, http.Network(c.GetHttp().GetNetwork()))
	}
	if c.GetHttp().GetAddr() != "" {
		opts = append(opts, http.Address(c.GetHttp().GetAddr()))
	}
	if c.GetHttp().GetTimeout() != nil {
		opts = append(opts, http.Timeout(c.GetHttp().GetTimeout().AsDuration()))
	}
	srv := http.NewServer(opts...)
	for _, r := range registrars {
		r.RegisterHTTP(srv)
	}
	return srv
}
