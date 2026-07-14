package runtimebinding

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/common"
	"github.com/wt-media/wt-media-cloud/internal/modules/cloudagent"
	"github.com/wt-media/wt-media-cloud/internal/modules/identity"
)

func RegisterRoutes(h *server.Hertz, service *Service, identityService *identity.Service) {
	h.POST("/api/v1/local-agent/binding-tickets", func(ctx context.Context, c *hertzapp.RequestContext) {
		auth, ok := identity.AuthenticateRequestContext(c, identityService)
		if !ok {
			return
		}
		grant, err := service.IssueTicket(auth.User, auth.Session.ID)
		if err != nil {
			writeRuntimeError(c, err)
			return
		}
		common.JSONData(c, consts.StatusCreated, grant)
	})

	h.POST("/api/v1/local-agent/nodes/register", func(ctx context.Context, c *hertzapp.RequestContext) {
		var input RegisterLocalInput
		if !common.DecodeJSON(c, &input) {
			return
		}
		registration, err := service.RegisterLocal(input)
		if err != nil {
			writeRuntimeError(c, err)
			return
		}
		common.JSONData(c, consts.StatusCreated, registration)
	})

	h.POST("/api/v1/local-agent/nodes/:node_id/runtime-report", func(ctx context.Context, c *hertzapp.RequestContext) {
		var report RuntimeReport
		if !common.DecodeJSON(c, &report) {
			return
		}
		credential, ok := bearerCredential(string(c.Request.Header.Peek("Authorization")))
		if !ok {
			writeRuntimeError(c, ErrNodeCredentialInvalid)
			return
		}
		if err := service.ReportRuntime(c.Param("node_id"), credential, report); err != nil {
			writeRuntimeError(c, err)
			return
		}
		common.JSONData(c, consts.StatusOK, map[string]string{"status": "reported"})
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

func writeRuntimeError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		common.JSONError(c, consts.StatusBadRequest, "invalid_runtime_report", "local Agent binding or runtime report is invalid")
	case errors.Is(err, ErrForbidden):
		common.JSONError(c, consts.StatusForbidden, "runtime_binding_forbidden", "local Agent binding is forbidden")
	case errors.Is(err, ErrBindingTicketInvalid):
		common.JSONError(c, consts.StatusUnauthorized, "binding_ticket_invalid", "local Agent binding ticket is invalid or expired")
	case errors.Is(err, ErrBoundSessionInvalid):
		common.JSONError(c, consts.StatusUnauthorized, "bound_session_invalid", "the local Agent user session is no longer active")
	case errors.Is(err, ErrNodeCredentialInvalid):
		common.JSONError(c, consts.StatusUnauthorized, "node_credential_invalid", "local Agent node credential is invalid")
	case errors.Is(err, ErrProfileOwnershipMismatch):
		common.JSONError(c, consts.StatusConflict, "profile_runtime_ownership_mismatch", "reported Profiles do not match the bound user and BitBrowser owner")
	case errors.Is(err, cloudagent.ErrIncompatibleAgent):
		common.JSONError(c, consts.StatusConflict, "incompatible_agent_contract", "Agent contract version is incompatible")
	default:
		common.JSONError(c, consts.StatusInternalServerError, "runtime_binding_store_error", "local Agent runtime binding operation failed")
	}
}
