package mediaaccount

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type createAccountRequest struct {
	UserID         string   `json:"user_id"`
	GameID         string   `json:"game_id"`
	Platform       Platform `json:"platform"`
	OriginalCookie string   `json:"original_cookie"`
}

type updateAccountRequest struct {
	BusinessStatus BusinessStatus `json:"business_status"`
	LoginStatus    LoginStatus    `json:"login_status"`
}

type identifyAccountRequest struct {
	PlatformAccountID string      `json:"platform_account_id"`
	Name              string      `json:"name"`
	AvatarURL         string      `json:"avatar_url"`
	LoginStatus       LoginStatus `json:"login_status"`
}

type tagsRequest struct {
	AccountIDs []string `json:"account_ids"`
	Tags       []string `json:"tags"`
}

type bindProfileRequest struct {
	BrowserProfileID string `json:"browser_profile_id"`
}

type TaskCreator interface {
	Create(cloudagent.CreateTaskRequest) cloudagent.Task
}

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service, taskStores ...TaskCreator) {
	var tasks TaskCreator
	if len(taskStores) > 0 {
		tasks = taskStores[0]
	}
	h.POST("/api/v1/media-accounts", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req createAccountRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		account, err := service.CreateAccount(actor, CreateAccountInput(req))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Created(c, account)
	})

	h.GET("/api/v1/media-accounts", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		accounts, err := service.ListAccounts(actor, AccountFilter{
			UserID:      c.Query("user_id"),
			GameID:      c.Query("game_id"),
			Platform:    Platform(c.Query("platform")),
			AnyTags:     commaValues(c.Query("any_tags")),
			AllTags:     commaValues(c.Query("all_tags")),
			ExcludeTags: commaValues(c.Query("exclude_tags")),
		})
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, accounts)
	})

	h.POST("/api/v1/media-accounts/tags/add", func(ctx context.Context, c *hertzapp.RequestContext) {
		changeTags(c, service, identityService, true)
	})
	h.POST("/api/v1/media-accounts/tags/remove", func(ctx context.Context, c *hertzapp.RequestContext) {
		changeTags(c, service, identityService, false)
	})

	h.GET("/api/v1/media-accounts/:account_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		account, err := service.GetAccount(actor, c.Param("account_id"))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, account)
	})

	h.PATCH("/api/v1/media-accounts/:account_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req updateAccountRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		account, err := service.UpdateAccount(actor, c.Param("account_id"), UpdateAccountInput(req))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, account)
	})

	h.POST("/api/v1/media-accounts/:account_id/identify", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req identifyAccountRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		account, err := service.IdentifyAccount(actor, c.Param("account_id"), IdentifyAccountInput(req))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, account)
	})

	h.POST("/api/v1/media-accounts/:account_id/check", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if tasks == nil {
			common.Failure(c, 503, 30006, "任务服务不可用", nil)
			return
		}
		record, err := service.GetAccountRecord(actor, c.Param("account_id"))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		if record.BrowserProfileID == "" {
			writeMediaAccountError(c, ErrProfileUnavailable)
			return
		}
		task := tasks.Create(cloudagent.CreateTaskRequest{
			TaskType:       cloudagent.TaskTypeAccountCheck.String(),
			IdempotencyKey: "account-check:" + actor.ID + ":" + record.ID + ":" + common.NewID("attempt"),
			Payload:        map[string]any{"account_id": record.ID, "profile_id": record.BrowserProfileID, "platform": record.Platform, "platform_account_id": record.PlatformAccountID},
		})
		common.Created(c, task)
	})

	h.POST("/api/v1/media-accounts/:account_id/cookies/read", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if tasks == nil {
			common.Failure(c, 503, 30006, "任务服务不可用", nil)
			return
		}
		record, err := service.GetAccountRecord(actor, c.Param("account_id"))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		if record.BrowserProfileID == "" {
			writeMediaAccountError(c, ErrProfileUnavailable)
			return
		}
		task := tasks.Create(cloudagent.CreateTaskRequest{TaskType: cloudagent.TaskTypeCookieRead.String(), IdempotencyKey: "cookie-read:" + actor.ID + ":" + record.ID + ":" + common.NewID("attempt"), Payload: map[string]any{"account_id": record.ID, "profile_id": record.BrowserProfileID, "platform": record.Platform}})
		common.Created(c, task)
	})

	// Cookie export: returns original and active cookie for the account.
	h.GET("/api/v1/media-accounts/:account_id/cookies", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		account, err := service.GetAccountRecord(actor, c.Param("account_id"))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, map[string]interface{}{
			"original_cookie": account.OriginalCookie,
			"active_cookie":   account.ActiveCookie,
		})
	})

	h.PATCH("/api/v1/media-accounts/:account_id/profile", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req bindProfileRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		account, err := service.BindProfile(actor, c.Param("account_id"), req.BrowserProfileID)
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, account)
	})
}

func changeTags(c *hertzapp.RequestContext, service *Service, identityService *identity.Service, add bool) {
	actor, ok := identity.AuthenticateRequest(c, identityService)
	if !ok {
		return
	}
	var req tagsRequest
	if !common.DecodeJSON(c, &req) {
		return
	}
	var err error
	if add {
		err = service.AddTags(actor, req.AccountIDs, req.Tags)
	} else {
		err = service.RemoveTags(actor, req.AccountIDs, req.Tags)
	}
	if err != nil {
		writeMediaAccountError(c, err)
		return
	}
	common.NoContent(c)
}

func commaValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func writeMediaAccountError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		common.Forbidden(c, 11003, "没有权限访问此媒体账号")
	case errors.Is(err, ErrNotFound):
		common.NotFound(c, 20004, "媒体账号不存在")
	case errors.Is(err, ErrDuplicateAccount):
		common.Conflict(c, 20009, "此平台账号已存在")
	case errors.Is(err, ErrProfilePlatformTaken):
		common.Conflict(c, 23001, "此浏览器环境已绑定其他平台账号")
	case errors.Is(err, ErrProfileUnavailable):
		common.Conflict(c, 23001, "浏览器环境不可用")
	case errors.Is(err, ErrInvalidInput):
		common.BadRequest(c, 10001, "媒体账号信息格式错误")
	default:
		common.InternalError(c, "媒体账号服务内部错误")
	}
}
