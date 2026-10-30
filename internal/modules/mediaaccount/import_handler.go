package mediaaccount

import (
	"context"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	service "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
	"strconv"
)

func PreviewMediaAccountImport(ctx context.Context, c *hertzapp.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.ImportInput
	if !decodeAccountJSON(c, &in) {
		return
	}
	out, e := service.PreviewImport(a, in)
	if e != nil {
		writeMediaAccountError(c, e)
		return
	}
	api.Success(c, out)
}
func CreateMediaAccountImport(ctx context.Context, c *hertzapp.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	var in service.ImportInput
	if !decodeAccountJSON(c, &in) {
		return
	}
	out, e := service.CreateImport(a, in)
	if e != nil {
		writeMediaAccountError(c, e)
		return
	}
	api.Created(c, out)
}
func ListMediaAccountImports(ctx context.Context, c *hertzapp.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	out, e := service.ListImports(a)
	if e != nil {
		writeMediaAccountError(c, e)
		return
	}
	api.Success(c, out)
}
func GetMediaAccountImport(ctx context.Context, c *hertzapp.RequestContext) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	out, e := service.GetImport(a, c.Param("batch_id"))
	if e != nil {
		writeMediaAccountError(c, e)
		return
	}
	api.Success(c, out)
}
func mutateImport(c *hertzapp.RequestContext, kind string) {
	a, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	row, e := strconv.Atoi(c.Param("row_number"))
	if e != nil || row <= 0 {
		writeMediaAccountError(c, service.ErrInvalidInput)
		return
	}
	var in service.ImportMutation
	if !decodeAccountJSON(c, &in) {
		return
	}
	out, e := service.MutateImportItem(a, c.Param("batch_id"), row, kind, in)
	if e != nil {
		writeMediaAccountError(c, e)
		return
	}
	api.Success(c, out)
}
func StartMediaAccountImportItem(ctx context.Context, c *hertzapp.RequestContext) {
	mutateImport(c, "start")
}
func SettleMediaAccountImportItem(ctx context.Context, c *hertzapp.RequestContext) {
	mutateImport(c, "result")
}
func ActMediaAccountImportItem(ctx context.Context, c *hertzapp.RequestContext) {
	mutateImport(c, "actions")
}
