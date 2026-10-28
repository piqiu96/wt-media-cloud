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
	"github.com/wt-media/wt-media-cloud/internal/modules/filetransfer"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/localagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount"
	"github.com/wt-media/wt-media-cloud/internal/modules/production"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy"
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
	registerModuleRoutes(engine)
	paths := config.GetRuntimePaths()
	return registerCloudWeb(engine, paths.Web)
}

// registerModuleRoutes 只安装各模块的 HTTP 路由。
//
// 单独拆出来是为了让测试能拿到完整模块列表：registerRoutes 会读配置并初始化管理员，
// 从它入口断言就等于在断言数据库。这里漏掉一个模块，它所有的路由都会 404，没有其他症状。
func registerModuleRoutes(engine *server.Hertz) {
	cloudagent.RegisterRoutes(engine)
	identity.RegisterRoutes(engine)
	localagent.RegisterRoutes(engine)
	profilebinding.RegisterRoutes(engine)
	mediaaccount.RegisterRoutes(engine)
	proxy.RegisterRoutes(engine)
	contentpool.RegisterRoutes(engine)
	production.RegisterRoutes(engine)
	filetransfer.RegisterRoutes(engine)
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
