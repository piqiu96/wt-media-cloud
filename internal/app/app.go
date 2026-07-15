package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/infra/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

type Server struct {
	engine *server.Hertz
	addr   string
	db     *sql.DB
}

func NewServer() (*Server, error) {
	cfg := config.Load()
	engine := server.Default(server.WithHostPorts(cfg.HTTPAddr))

	registerHealthRoutes(engine)

	result := &Server{engine: engine, addr: cfg.HTTPAddr}

	var taskStore *cloudagent.MySQLTaskStore
	var agentRegistry *cloudagent.MySQLRegistry

	if cfg.MySQLDSN != "" {
		db, err := database.OpenMySQL(cfg.MySQLDSN)
		if err != nil {
			return nil, fmt.Errorf("open mysql: %w", err)
		}
		result.db = db
		taskStore = cloudagent.NewMySQLTaskStore(db)
		agentRegistry = cloudagent.NewMySQLRegistry(db)
	} else {
		taskStore = cloudagent.NewMySQLTaskStore(nil)
		agentRegistry = cloudagent.NewMySQLRegistry(nil)
	}

	cloudagent.RegisterRoutes(engine, agentRegistry, taskStore)

	if result.db != nil {
		identityService := identity.NewService(identity.NewMySQLStore(result.db))
		if err := bootstrapIdentity(identityService, cfg.InitialTechnicianUsername, cfg.InitialTechnicianPassword); err != nil {
			result.db.Close()
			return nil, err
		}
		identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: cfg.SessionCookieSecure})
		runtimeService := runtimebinding.NewService(runtimebinding.NewMySQLStore(result.db))
		runtimebinding.RegisterRoutes(engine, runtimeService, identityService)
		profileguard.RegisterRoutes(engine, profileguard.NewService(profileguard.NewMySQLStore(result.db), runtimeService))
		profileStore := profilebinding.NewMySQLStore(result.db)
		profilebinding.RegisterRoutes(engine, profilebinding.NewService(profileStore), identityService)
		mediaaccount.RegisterRoutes(engine, mediaaccount.NewService(mediaaccount.NewMySQLStore(result.db), mediaaccount.WithProfileResolver(profileStore)), identityService)
	} else if cfg.InitialTechnicianUsername != "" || cfg.InitialTechnicianPassword != "" {
		return nil, fmt.Errorf("identity bootstrap requires WT_MEDIA_MYSQL_DSN")
	}
	return result, nil
}

func (s *Server) Run() error {
	log.Printf("wt-media-cloud listening on %s", s.addr)
	s.engine.Spin()
	return nil
}

func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func bootstrapIdentity(service *identity.Service, username, password string) error {
	if username == "" && password == "" {
		return nil
	}
	if username == "" || password == "" {
		return fmt.Errorf("both initial technician username and password are required")
	}
	_, err := service.BootstrapTechnician(username, password)
	if errors.Is(err, identity.ErrBootstrapUnavailable) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("bootstrap initial technician: %w", err)
	}
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
