package production

import (
	"context"
	"errors"
	"strconv"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

// ListComposeOperators answers the strategy form's 负责人 picker. It is a
// production endpoint on purpose: the identity module's /users permission does
// not widen for senior operators just because strategy assignment needs names.
func ListComposeOperators(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	var teamID int64
	if raw := strings.TrimSpace(c.Query("team_id")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			api.FailureNamed(c, consts.StatusBadRequest, 15220, "团队 ID 无效", "compose_operator_invalid")
			return
		}
		teamID = value
	}
	categoryID, err := parseOptionalCategoryID(c.Query("category_id"))
	if err != nil {
		api.FailureNamed(c, consts.StatusBadRequest, 15220, "类目 ID 无效", "compose_operator_invalid")
		return
	}
	options, err := productionservice.ListComposeOperators(current, teamID, categoryID)
	if err != nil {
		writeComposeOperatorError(c, err)
		return
	}
	api.Success(c, options)
}

func writeComposeOperatorError(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, productionservice.ErrComposeOperatorForbidden):
		api.ForbiddenNamed(c, 15221, "没有权限查看可指派负责人", "compose_operator_forbidden")
	case errors.Is(err, productionservice.ErrComposeOperatorInvalid):
		api.FailureNamed(c, consts.StatusBadRequest, 15220, err.Error(), "compose_operator_invalid")
	default:
		api.InternalError(c, "负责人目录服务内部错误")
	}
}
