package profilebinding

import (
	"context"
	"errors"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service) {
	h.POST("/api/v1/bit-browser/profile-scans", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		var input SnapshotInput
		if !common.DecodeJSON(c, &input) {
			return
		}
		scan, err := service.SubmitScan(actor, input)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.JSONData(c, consts.StatusCreated, scan)
	})
	h.GET("/api/v1/bit-browser/profile-scans/:scan_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		scan, err := service.GetScan(actor, c.Param("scan_id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.JSONData(c, consts.StatusOK, scan)
	})
	h.POST("/api/v1/bit-browser/profile-scans/:scan_id/confirm", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		scan, err := service.ConfirmScan(actor, c.Param("scan_id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.JSONData(c, consts.StatusOK, scan)
	})
	h.GET("/api/v1/browser-profiles", func(ctx context.Context, c *hertzapp.RequestContext) {
		actor, ok := identity.AuthenticateRequest(c, identityService)
		if !ok {
			return
		}
		profiles, err := service.ListProfiles(actor, c.Query("user_id"))
		if err != nil {
			writeProfileError(c, err)
			return
		}
		common.JSONData(c, consts.StatusOK, profiles)
	})
}

func writeProfileError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		common.JSONError(c, consts.StatusForbidden, "forbidden", "the current user cannot perform this Profile operation")
	case errors.Is(err, ErrIdentityUnverifiable):
		common.JSONError(c, consts.StatusConflict, "bitbrowser_identity_unverifiable", "BitBrowser Profile identity cannot be verified")
	case errors.Is(err, ErrIdentityMismatch):
		common.JSONError(c, consts.StatusConflict, "bitbrowser_identity_mismatch", "BitBrowser identity does not match the bound user")
	case errors.Is(err, ErrScanNotFound), errors.Is(err, ErrProfileNotFound):
		common.JSONError(c, consts.StatusNotFound, "profile_scan_not_found", "Profile scan or Profile was not found")
	case errors.Is(err, ErrScanExpired):
		common.JSONError(c, consts.StatusGone, "profile_scan_expired", "Profile scan has expired")
	case errors.Is(err, ErrScanNotReady):
		common.JSONError(c, consts.StatusConflict, "profile_scan_not_ready", "Profile scan is not ready")
	default:
		common.JSONError(c, consts.StatusInternalServerError, "profile_store_error", "Profile operation failed")
	}
}
