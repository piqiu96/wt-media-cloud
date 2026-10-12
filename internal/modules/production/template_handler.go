package production

import (
	"context"
	"errors"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func ListComposeTemplates(_ context.Context, c *hertzapp.RequestContext) {
	current, ok := actor(c)
	if !ok {
		return
	}
	items, err := productionservice.ListComposeTemplates(current)
	switch {
	case errors.Is(err, productionservice.ErrTemplateForbidden):
		api.ForbiddenNamed(c, 15202, "没有权限查看合成模板", "compose_template_forbidden")
	case err != nil:
		api.InternalError(c, "合成模板目录读取失败")
	default:
		api.Success(c, items)
	}
}
