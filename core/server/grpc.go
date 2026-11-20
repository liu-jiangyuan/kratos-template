package server

import (
	"log/slog"

	"github.com/liu-jiangyuan/kratos-template/core/conf"
	"github.com/liu-jiangyuan/kratos-template/core/middleware/requestid"

	"github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	"github.com/go-kratos/kratos/v3/middleware/logging"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/grpc"
)

// NewGRPCServer creates the shared gRPC server and registers all module routes.
func NewGRPCServer(c *conf.Server, logger *slog.Logger, registrars []Registrar) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			tracing.Server(),
			requestid.Server(),
			logging.Server(logger),
		),
	}
	if c.GetGrpc().GetNetwork() != "" {
		opts = append(opts, grpc.Network(c.GetGrpc().GetNetwork()))
	}
	if c.GetGrpc().GetAddr() != "" {
		opts = append(opts, grpc.Address(c.GetGrpc().GetAddr()))
	}
	if c.GetGrpc().GetTimeout() != nil {
		opts = append(opts, grpc.Timeout(c.GetGrpc().GetTimeout().AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	for _, r := range registrars {
		r.RegisterGRPC(srv)
	}
	return srv
}
