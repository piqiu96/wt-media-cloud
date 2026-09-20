package bootstrap

import (
	"context"
	"errors"
	"fmt"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func registerRoutes(engine *server.Hertz) error {
	engine.Use(middleware.LocalDesktopCORS())
	engine.Use(middleware.RequestContext())
	engine.Use(middleware.IdentityContext())
	registerHealthRoutes(engine)

	cfg := config.Get()
	if err := bootstrapIdentity(cfg.App.InitialAdmin.Username, cfg.App.InitialAdmin.Password); err != nil {
		return err
	}
	cloudagent.RegisterRoutes(engine)
	identity.RegisterRoutes(engine)
	runtimebinding.RegisterRoutes(engine)
	profileguard.RegisterRoutes(engine)
	profilebinding.RegisterRoutes(engine)
	mediaaccount.RegisterRoutes(engine)
	proxy.RegisterRoutes(engine)
	contentpool.RegisterRoutes(engine)
	return nil
}

func bootstrapIdentity(username, password string) error {
	if username == "" && password == "" {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("both initial admin username and password are required")
	}
	_, err := identityservice.BootstrapAdmin(username, password)
	if errors.Is(err, identityservice.ErrBootstrapUnavailable) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("bootstrap initial admin: %w", err)
	}
	return nil
}

func registerHealthRoutes(h *server.Hertz) {
	h.GET("/healthz", func(_ context.Context, c *hertzapp.RequestContext) { c.String(consts.StatusOK, "ok") })
	h.GET("/api/v1/health", func(_ context.Context, c *hertzapp.RequestContext) {
		api.Success(c, map[string]string{"status": "ok"})
	})
}
