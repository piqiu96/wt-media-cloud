package app

import (
	"context"
	"log"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/infra/config"
)

type Server struct {
	engine *server.Hertz
	addr   string
}

func NewServer() *Server {
	cfg := config.Load()
	engine := server.Default(server.WithHostPorts(cfg.HTTPAddr))

	registerHealthRoutes(engine)

	return &Server{engine: engine, addr: cfg.HTTPAddr}
}

func (s *Server) Run() error {
	log.Printf("wt-media-cloud listening on %s", s.addr)
	s.engine.Spin()
	return nil
}

func registerHealthRoutes(h *server.Hertz) {
	h.GET("/healthz", func(ctx context.Context, c *hertzapp.RequestContext) {
		c.String(consts.StatusOK, "ok")
	})
	h.GET("/api/v1/health", func(ctx context.Context, c *hertzapp.RequestContext) {
		common.JSONData(c, consts.StatusOK, map[string]string{"status": "ok"})
	})
}
