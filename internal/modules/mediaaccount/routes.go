package mediaaccount

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

type createAccountRequest struct {
	UserID           identity.UserID `json:"user_id"`
	GameIDs          []string        `json:"game_ids"`
	GameID           string          `json:"game_id"`
	Name             string          `json:"name"`
	Platform         Platform        `json:"platform"`
	OriginalCookie   string          `json:"original_cookie"`
	BrowserProfileID string          `json:"browser_profile_id"`
	Remark           string          `json:"remark"`
	Tags             []string        `json:"tags"`
}

type updateAccountRequest struct {
	BusinessStatus BusinessStatus `json:"business_status"`
	LoginStatus    LoginStatus    `json:"login_status"`
	Remark         *string        `json:"remark"`
	GameIDs        *[]string      `json:"game_ids"`
	GameID         *string        `json:"game_id"`
	Name           *string        `json:"name"`
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

type startAccountCheckRequest struct {
	NodeID string `json:"node_id"`
}

type accountCheckResultRequest struct {
	TaskID            string             `json:"task_id"`
	PlatformAccountID string             `json:"platform_account_id"`
	Name              string             `json:"name"`
	AvatarURL         string             `json:"avatar_url"`
	LoginStatus       LoginStatus        `json:"login_status"`
	Message           string             `json:"message"`
	CheckItems        []AccountCheckItem `json:"check_items"`
}

type startCookieReadRequest struct {
	NodeID string `json:"node_id"`
}

type cookieReadResultRequest struct {
	TaskID  string           `json:"task_id"`
	Cookies []map[string]any `json:"cookies"`
}

type createAccountGroupRequest struct {
	Name    string              `json:"name"`
	Filters AccountGroupFilters `json:"filters"`
}

type updateAccountGroupRequest struct {
	Name    *string              `json:"name"`
	Filters *AccountGroupFilters `json:"filters"`
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
		gameIDs, err := resolveCreateRequestGameIDs(req.GameIDs, req.GameID)
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		account, err := service.CreateAccount(actor, CreateAccountInput{
			UserID: req.UserID, GameIDs: gameIDs, Name: req.Name, Platform: req.Platform,
			OriginalCookie: req.OriginalCookie, BrowserProfileID: req.BrowserProfileID, Remark: req.Remark, Tags: req.Tags,
		})
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
		var userID identity.UserID
		if raw := c.Query("user_id"); raw != "" {
			parsed, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || parsed <= 0 {
				writeMediaAccountError(c, ErrInvalidInput)
				return
			}
			userID = identity.UserID(parsed)
		}
		gameIDs, err := resolveQueryGameIDs(commaValues(c.Query("game_ids")), c.Query("game_id"))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		accounts, err := service.ListAccounts(actor, AccountFilter{
			UserID:         userID,
			GameIDs:        gameIDs,
			Platform:       Platform(c.Query("platform")),
			BusinessStatus: BusinessStatus(c.Query("business_status")),
			LoginStatus:    LoginStatus(c.Query("login_status")),
			Search:         c.Query("search"),
			ProfileSearch:  c.Query("profile_search"),
			AnyTags:        commaValues(c.Query("any_tags")),
			AllTags:        commaValues(c.Query("all_tags")),
			ExcludeTags:    commaValues(c.Query("exclude_tags")),
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
		gameIDs, err := resolveUpdateRequestGameIDs(req.GameIDs, req.GameID)
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		account, err := service.UpdateAccount(actor, c.Param("account_id"), UpdateAccountInput{
			BusinessStatus: req.BusinessStatus, LoginStatus: req.LoginStatus, Remark: req.Remark, GameIDs: gameIDs, Name: req.Name,
		})
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
		var req startAccountCheckRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		start, err := service.StartLocalAccountCheck(actor, c.Param("account_id"), AccountCheckStartInput(req))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Created(c, start)
	})

	h.POST("/api/v1/media-accounts/:account_id/check/result", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req accountCheckResultRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		account, err := service.ApplyLocalAccountCheckResult(actor, c.Param("account_id"), AccountCheckResultInput(req))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, account)
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
		record, err := service.GetOwnedAccountRecord(actor, c.Param("account_id"))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		if record.BrowserProfileID == "" {
			writeMediaAccountError(c, ErrProfileUnavailable)
			return
		}
		task := tasks.Create(cloudagent.CreateTaskRequest{TaskType: cloudagent.TaskTypeCookieRead.String(), IdempotencyKey: "cookie-read:" + strconv.FormatInt(int64(actor.ID), 10) + ":" + record.ID + ":" + common.NewID("attempt"), Payload: map[string]any{"account_id": record.ID, "profile_id": record.BrowserProfileID, "platform": record.Platform}})
		common.Created(c, task)
	})

	// 同步 Cookie 读回：创建敏感任务（互斥），Desktop 经 Tauri preflight→Agent 读回→result 应用
	h.POST("/api/v1/media-accounts/:account_id/cookies/read-sync", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req startCookieReadRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		start, err := service.StartCookieRead(actor, c.Param("account_id"), CookieReadStartInput{NodeID: req.NodeID})
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Created(c, start)
	})

	h.POST("/api/v1/media-accounts/:account_id/cookies/read-sync/result", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req cookieReadResultRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		account, err := service.ApplyCookieReadResult(actor, c.Param("account_id"), CookieReadResultInput{TaskID: req.TaskID, Cookies: req.Cookies})
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, account)
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

	h.DELETE("/api/v1/media-accounts/:account_id/profile", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		account, err := service.UnbindProfile(actor, c.Param("account_id"))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, account)
	})

	// 账号组（可保存筛选，PRD 3.3.6）
	h.POST("/api/v1/account-groups", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req createAccountGroupRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		group, err := service.CreateAccountGroup(actor, CreateAccountGroupInput{Name: req.Name, Filters: req.Filters})
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Created(c, group)
	})

	h.GET("/api/v1/account-groups", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		groups, err := service.ListAccountGroups(actor)
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, groups)
	})

	h.GET("/api/v1/account-groups/:group_id/accounts", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		accounts, err := service.ListAccountsByGroup(actor, c.Param("group_id"))
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, accounts)
	})

	h.PATCH("/api/v1/account-groups/:group_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var req updateAccountGroupRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		group, err := service.UpdateAccountGroup(actor, c.Param("group_id"), UpdateAccountGroupInput{Name: req.Name, Filters: req.Filters})
		if err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, group)
	})

	h.DELETE("/api/v1/account-groups/:group_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		if err := service.DeleteAccountGroup(actor, c.Param("group_id")); err != nil {
			writeMediaAccountError(c, err)
			return
		}
		common.Success(c, nil)
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

func resolveCreateRequestGameIDs(canonical []string, compatibility string) ([]string, error) {
	if canonical == nil {
		if strings.TrimSpace(compatibility) == "" {
			return nil, nil
		}
		return []string{compatibility}, nil
	}
	if strings.TrimSpace(compatibility) == "" {
		return canonical, nil
	}
	normalized := normalizeGameIDs(canonical)
	if len(normalized) != 1 || normalized[0] != strings.TrimSpace(compatibility) {
		return nil, ErrInvalidInput
	}
	return canonical, nil
}

func resolveUpdateRequestGameIDs(canonical *[]string, compatibility *string) (*[]string, error) {
	if canonical == nil {
		if compatibility == nil {
			return nil, nil
		}
		result := []string{*compatibility}
		if strings.TrimSpace(*compatibility) == "" {
			result = []string{}
		}
		return &result, nil
	}
	if compatibility == nil {
		return canonical, nil
	}
	if !sameGameIDs(normalizeGameIDs(*canonical), normalizeGameIDs([]string{*compatibility})) {
		return nil, ErrInvalidInput
	}
	return canonical, nil
}

func resolveQueryGameIDs(canonical []string, compatibility string) ([]string, error) {
	if len(canonical) == 0 {
		if strings.TrimSpace(compatibility) == "" {
			return nil, nil
		}
		return []string{compatibility}, nil
	}
	if strings.TrimSpace(compatibility) == "" {
		return canonical, nil
	}
	if !sameGameIDs(normalizeGameIDs(canonical), normalizeGameIDs([]string{compatibility})) {
		return nil, ErrInvalidInput
	}
	return canonical, nil
}

func sameGameIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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
