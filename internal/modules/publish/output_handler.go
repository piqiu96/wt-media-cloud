package publish

import (
	"context"
	app "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"strconv"
)

func parseOutputCandidateQuery(c *app.RequestContext) (production.ListComposeOutputsInput, error) {
	in := production.ListComposeOutputsInput{Page: 1, PageSize: 10, Search: c.Query("search"), Platform: c.Query("platform")}
	for key, target := range map[string]*int{"page": &in.Page, "page_size": &in.PageSize} {
		if raw := c.Query(key); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || key == "page_size" && n > 100 {
				return in, service.ErrInvalid
			}
			*target = n
		}
	}
	if raw := c.Query("category_id"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			return in, service.ErrInvalid
		}
		in.CategoryID = identity.CategoryID(n)
	}
	return in, nil
}
func OutputCandidates(ctx context.Context, c *app.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	in, err := parseOutputCandidateQuery(c)
	if err != nil {
		fail(c, err)
		return
	}
	page, err := service.Wired().OutputCandidates(ctx, actor, in)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, page)
}
