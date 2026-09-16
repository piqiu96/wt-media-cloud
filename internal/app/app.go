package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/infra/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/scheduler"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/contentpool"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding"
	"github.com/wt-media/wt-media-cloud/internal/modules/profileguard"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy"
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

	// Request logging and trace ID middleware.
	engine.Use(localDesktopCORSMiddleware())
	engine.Use(traceAndLogMiddleware())

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
		if err := bootstrapIdentity(identityService, cfg.InitialAdminUsername, cfg.InitialAdminPassword); err != nil {
			result.db.Close()
			return nil, err
		}
		identity.RegisterRoutes(engine, identityService, identity.RouteConfig{CookieSecure: cfg.SessionCookieSecure})
		runtimeService := runtimebinding.NewService(runtimebinding.NewMySQLStore(result.db))
		runtimebinding.RegisterRoutes(engine, runtimeService, identityService)
		profileGuardStore := profileguard.NewMySQLStore(result.db)
		profileguard.RegisterRoutes(engine, profileguard.NewService(profileGuardStore, runtimeService))
		profileStore := profilebinding.NewMySQLStore(result.db)
		profileService := profilebinding.NewService(profileStore)
		profilebinding.RegisterRoutes(engine, profileService, identityService, taskStore, runtimeService)
		mediaaccount.RegisterRoutes(engine, mediaaccount.NewService(
			mediaaccount.NewMySQLStore(result.db),
			mediaaccount.WithProfileResolver(profileStore),
			mediaaccount.WithProfileFactResolver(profileStore),
			mediaaccount.WithSensitiveTaskCreator(profileGuardStore),
			mediaaccount.WithUserResolver(identityService),
			mediaaccount.WithGameResolver(identityService),
		), identityService, taskStore)
		proxy.RegisterRoutes(engine, proxy.NewService(proxy.NewMySQLStore(result.db)), identityService, taskStore, profileStore, proxy.NewHTTPAgentCheckerFromEnv(), profileService, runtimeService)
		contentPoolService := contentpool.NewService(contentpool.NewMySQLStore(result.db))
		contentpool.RegisterRoutes(engine, contentPoolService, identityService)
		discoveryService := contentpool.NewDiscoveryService(contentpool.NewMySQLDiscoveryStore(result.db), contentPoolService)
		discoveryService.SetTaskCreator(taskStore.Create)
		taskStore.SetResultHandler(func(task cloudagent.Task) error {
			if task.TaskType != cloudagent.TaskTypeDiscovery.String() {
				return nil
			}
			if task.Status == cloudagent.TaskStatusRunning.String() {
				return discoveryService.HandleTaskProgress(task.TaskID, contentpool.CrawlRunning, task.Message)
			}
			if task.Status == cloudagent.TaskStatusFailed.String() {
				return discoveryService.HandleTaskProgress(task.TaskID, contentpool.CrawlFailed, task.Message)
			}
			result := task.Result
			if result == nil {
				result = map[string]any{}
			}
			if _, ok := result["crawl_task_id"]; !ok && task.Payload != nil {
				result["crawl_task_id"] = task.Payload["crawl_task_id"]
			}
			return discoveryService.HandleTaskResult(task.TaskID, result)
		})
		contentpool.RegisterDiscoveryRoutes(engine, discoveryService, identityService)
		scheduler.StartDiscoveryScheduler(discoveryService)
	} else if cfg.InitialAdminUsername != "" || cfg.InitialAdminPassword != "" {
		return nil, fmt.Errorf("identity bootstrap requires WT_MEDIA_MYSQL_DSN")
	}

	// Start background schedulers
	if result.db != nil {
		scheduler.StartProxyExpiryChecker(result.db, 6*time.Hour)
	}

	return result, nil
}

func (s *Server) Run() error {
	hlog.Infof("wt-media-cloud listening on %s", s.addr)
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
		return fmt.Errorf("both initial admin username and password are required")
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
	h.GET("/healthz", func(ctx context.Context, c *hertzapp.RequestContext) {
		c.String(consts.StatusOK, "ok")
	})
	h.GET("/api/v1/health", func(ctx context.Context, c *hertzapp.RequestContext) {
		common.Success(c, map[string]string{"status": "ok"})
	})
}

// traceAndLogMiddleware injects a trace ID and logid into each request and logs it.
func traceAndLogMiddleware() hertzapp.HandlerFunc {
	return func(ctx context.Context, c *hertzapp.RequestContext) {
		start := time.Now()
		traceID := generateTraceID()
		c.Set("trace_id", traceID)
		c.Set("logid", traceID)

		c.Next(ctx)

		status := c.Response.StatusCode()
		elapsed := time.Since(start)
		hlog.CtxInfof(ctx, "[%s] %s %s %d %v", traceID, string(c.Method()), string(c.Path()), status, elapsed)
	}
}

func localDesktopCORSMiddleware() hertzapp.HandlerFunc {
	return func(ctx context.Context, c *hertzapp.RequestContext) {
		origin := string(c.Request.Header.Peek("Origin"))
		if isAllowedLocalDesktopOrigin(origin) {
			c.Response.Header.Set("Access-Control-Allow-Origin", origin)
			c.Response.Header.Set("Access-Control-Allow-Credentials", "true")
			c.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Token")
			c.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Response.Header.Set("Vary", "Origin")
		}
		if string(c.Method()) == consts.MethodOptions {
			c.SetStatusCode(consts.StatusNoContent)
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}

func isAllowedLocalDesktopOrigin(origin string) bool {
	switch strings.TrimSpace(origin) {
	case "http://tauri.localhost",
		"https://tauri.localhost",
		"tauri://localhost",
		"http://127.0.0.1:5174",
		"http://localhost:5174":
		return true
	default:
		return false
	}
}

func generateTraceID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "lg" + hex.EncodeToString(bytes)
}
