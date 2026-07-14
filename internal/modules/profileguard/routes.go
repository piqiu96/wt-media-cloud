package profileguard

import (
	"context"
	"errors"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding"
)

type nodeRequest struct {
	NodeID string `json:"node_id"`
}
type renewRequest struct {
	NodeID       string `json:"node_id"`
	LeaseSeconds int    `json:"lease_seconds"`
}
type finishRequest struct {
	NodeID  string        `json:"node_id"`
	Outcome FinishOutcome `json:"outcome"`
}

func RegisterRoutes(h *server.Hertz, service *Service) {
	h.POST("/api/v1/local-agent/sensitive-tasks/:task_id/preflight", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req nodeRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
		if !ok {
			writeGuardError(c, runtimebinding.ErrNodeCredentialInvalid)
			return
		}
		outcome, err := service.Preflight(req.NodeID, credential, c.Param("task_id"))
		if err != nil {
			writeGuardError(c, err)
			return
		}
		common.JSONData(c, consts.StatusOK, outcome)
	})

	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/renew", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req renewRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		nodeCredential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
		permitCredential := strings.TrimSpace(string(c.Request.Header.Peek("X-Profile-Permit")))
		if !ok || permitCredential == "" {
			writeGuardError(c, ErrPermitCredentialInvalid)
			return
		}
		expiresAt, err := service.Renew(req.NodeID, nodeCredential, c.Param("permit_id"), permitCredential, time.Duration(req.LeaseSeconds)*time.Second)
		if err != nil {
			writeGuardError(c, err)
			return
		}
		common.JSONData(c, consts.StatusOK, map[string]any{"status": "renewed", "expires_at": expiresAt})
	})

	h.POST("/api/v1/local-agent/sensitive-permits/:permit_id/finish", func(ctx context.Context, c *hertzapp.RequestContext) {
		var req finishRequest
		if !common.DecodeJSON(c, &req) {
			return
		}
		nodeCredential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
		permitCredential := strings.TrimSpace(string(c.Request.Header.Peek("X-Profile-Permit")))
		if !ok || permitCredential == "" {
			writeGuardError(c, ErrPermitCredentialInvalid)
			return
		}
		if err := service.Finish(req.NodeID, nodeCredential, c.Param("permit_id"), permitCredential, req.Outcome); err != nil {
			writeGuardError(c, err)
			return
		}
		common.JSONData(c, consts.StatusOK, map[string]string{"status": string(req.Outcome)})
	})
}

func bearerCredential(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return value, value != ""
}

func writeGuardError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, runtimebinding.ErrNodeCredentialInvalid), errors.Is(err, runtimebinding.ErrBoundSessionInvalid), errors.Is(err, ErrPermitCredentialInvalid):
		common.JSONError(c, consts.StatusUnauthorized, "sensitive_credential_invalid", "node, session, or Profile permit credential is invalid")
	case errors.Is(err, ErrInvalidInput):
		common.JSONError(c, consts.StatusBadRequest, "invalid_sensitive_preflight", "sensitive task preflight request is invalid")
	case errors.Is(err, ErrTaskNotFound):
		common.JSONError(c, consts.StatusNotFound, "sensitive_task_not_found", "sensitive task authorization was not found")
	case errors.Is(err, ErrTaskAssignmentMismatch):
		common.JSONError(c, consts.StatusForbidden, "sensitive_task_assignment_mismatch", "sensitive task is not assigned to this node and user")
	case errors.Is(err, ErrRuntimeUnavailable):
		common.JSONError(c, consts.StatusConflict, "profile_runtime_unavailable", "fresh matching Profile runtime presence is unavailable")
	default:
		common.JSONError(c, consts.StatusInternalServerError, "profile_guard_store_error", "sensitive Profile guard operation failed")
	}
}
