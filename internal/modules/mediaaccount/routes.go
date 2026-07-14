package mediaaccount

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
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

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service) {
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
		common.JSONData(c, consts.StatusCreated, account)
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
		common.JSONData(c, consts.StatusOK, accounts)
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
		common.JSONData(c, consts.StatusOK, account)
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
		common.JSONData(c, consts.StatusOK, account)
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
		common.JSONData(c, consts.StatusOK, account)
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
		common.JSONData(c, consts.StatusOK, account)
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
	common.JSONData(c, consts.StatusOK, map[string]string{"status": "updated"})
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
		common.JSONError(c, consts.StatusForbidden, "forbidden", "the current user cannot access this media account")
	case errors.Is(err, ErrNotFound):
		common.JSONError(c, consts.StatusNotFound, "media_account_not_found", "media account was not found")
	case errors.Is(err, ErrDuplicateAccount):
		common.JSONError(c, consts.StatusConflict, "duplicate_media_account", "this user already has the platform account")
	case errors.Is(err, ErrProfilePlatformTaken):
		common.JSONError(c, consts.StatusConflict, "profile_platform_account_taken", "this Profile already has an account for the platform")
	case errors.Is(err, ErrProfileUnavailable):
		common.JSONError(c, consts.StatusConflict, "browser_profile_unavailable", "browser Profile is missing or inactive")
	case errors.Is(err, ErrInvalidInput):
		common.JSONError(c, consts.StatusBadRequest, "invalid_media_account_request", "media account request is invalid")
	default:
		common.JSONError(c, consts.StatusInternalServerError, "media_account_store_error", "media account operation failed")
	}
}
