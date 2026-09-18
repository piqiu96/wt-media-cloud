package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
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
	registerHealthRoutes(engine)

	cfg := config.Get()
	db := legacySQLDB()
	taskStore := cloudagent.NewMySQLTaskStore(db)
	agentRegistry := cloudagent.NewMySQLRegistry(db)
	cloudagent.RegisterRoutes(engine, agentRegistry, taskStore)

	identityService := identity.NewService(identity.NewMySQLStore(db))
	if err := bootstrapIdentity(identityService, cfg.App.InitialAdmin.Username, cfg.App.InitialAdmin.Password); err != nil {
		return err
	}
	identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: cfg.App.Server.SessionCookieSecure})

	runtimeService := runtimebinding.NewService(runtimebinding.NewMySQLStore(db))
	runtimebinding.RegisterRoutes(engine, runtimeService, identityService)
	profileGuardStore := profileguard.NewMySQLStore(db)
	profileguard.RegisterRoutes(engine, profileguard.NewService(profileGuardStore, runtimeService))
	profileStore := profilebinding.NewMySQLStore(db)
	profileService := profilebinding.NewService(profileStore)
	profilebinding.RegisterRoutes(engine, profileService, identityService, taskStore, runtimeService)
	mediaaccount.RegisterRoutes(
		engine,
		mediaaccount.NewService(
			mediaaccount.NewMySQLStore(db),
			mediaaccount.WithProfileResolver(profileStore),
			mediaaccount.WithProfileFactResolver(profileStore),
			mediaaccount.WithSensitiveTaskCreator(profileGuardStore),
			mediaaccount.WithUserResolver(identityService),
			mediaaccount.WithGameResolver(identityService),
		),
		identityService,
		taskStore,
	)
	proxy.RegisterRoutes(
		engine,
		proxy.NewService(proxy.NewMySQLStore(db)),
		identityService,
		taskStore,
		profileStore,
		proxy.NewAgentChecker(),
		profileService,
		runtimeService,
	)

	contentService := contentpool.NewService(contentpool.NewMySQLStore(db))
	contentpool.RegisterRoutes(engine, contentService, identityService)
	discoveryService := contentpool.NewDiscoveryService(
		contentpool.NewMySQLDiscoveryStore(db),
		contentService,
		contentpool.NewDouyinCrawler(),
	)
	contentpool.RegisterDiscoveryRoutes(engine, discoveryService, identityService)
	return nil
}

func bootstrapIdentity(service *identity.Service, username, password string) error {
	if username == "" && password == "" {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("both initial admin username and password are required")
	}
	_, err := service.BootstrapAdmin(username, password)
	if errors.Is(err, identity.ErrBootstrapUnavailable) {
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

func legacySQLDB() *sql.DB {
	sqlDB, err := database.DB().DB()
	if err != nil {
		panic(fmt.Sprintf("bootstrap: primary database is not backed by *sql.DB: %v", err))
	}
	return sqlDB
}

func configServerAddr() string {
	return config.Get().App.Server.HTTPAddr
}
