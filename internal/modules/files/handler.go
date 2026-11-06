package files

import (
	"context"
	"errors"
	"strconv"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/middleware"
	fileservice "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	api "github.com/wt-media/wt-media-cloud/internal/shared/api"
)

func RegisterRoutes(h *server.Hertz) {
	h.GET("/api/v1/files", List)
	h.POST("/api/v1/files", Upload)
	h.PATCH("/api/v1/files/:file_id", Rename)
	h.DELETE("/api/v1/files/:file_id", Delete)
	h.GET("/api/v1/file-resources/:token", Resource)
}
func fail(c *hertzapp.RequestContext, err error) {
	switch {
	case errors.Is(err, fileservice.ErrInvalid):
		api.FailureNamed(c, consts.StatusBadRequest, 17001, "请填写有效文件名并选择可读取的图片或视频", "file_invalid")
	case errors.Is(err, fileservice.ErrTooLarge):
		api.FailureNamed(c, consts.StatusRequestEntityTooLarge, 17002, "文件超过上传大小限制", "file_too_large")
	case errors.Is(err, fileservice.ErrNotFound):
		api.FailureNamed(c, consts.StatusNotFound, 17003, "文件不存在或没有访问权限", "file_not_found")
	case errors.Is(err, fileservice.ErrForbidden):
		api.FailureNamed(c, consts.StatusForbidden, 17004, "没有权限操作文件", "file_forbidden")
	default:
		api.FailureNamed(c, consts.StatusServiceUnavailable, 17005, "文件操作暂未完成，请稍后重试", "file_unavailable")
	}
}
func wired(c *hertzapp.RequestContext) (*fileservice.Service, bool) {
	s, err := fileservice.Wired()
	if err != nil {
		fail(c, err)
		return nil, false
	}
	return s, true
}
func fileID(c *hertzapp.RequestContext) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("file_id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, fileservice.ErrInvalid)
		return 0, false
	}
	return id, true
}
func List(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	s, ok := wired(c)
	if !ok {
		return
	}
	page, size := 1, 20
	for key, target := range map[string]*int{"page": &page, "page_size": &size} {
		if raw := c.Query(key); raw != "" {
			v, err := strconv.Atoi(raw)
			if err != nil {
				fail(c, fileservice.ErrInvalid)
				return
			}
			*target = v
		}
	}
	items, total, err := s.List(actor, c.Query("type"), c.Query("search"), page, size)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, map[string]any{"items": items, "total": total, "page": page, "page_size": size, "upload_max_bytes": s.MaxBytes})
}
func Upload(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	s, ok := wired(c)
	if !ok {
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		fail(c, fileservice.ErrInvalid)
		return
	}
	defer form.RemoveAll()
	files := form.File["file"]
	names := form.Value["name"]
	if len(files) != 1 || len(names) != 1 {
		fail(c, fileservice.ErrInvalid)
		return
	}
	file := files[0]
	if file.Size > s.MaxBytes {
		fail(c, fileservice.ErrTooLarge)
		return
	}
	body, err := file.Open()
	if err != nil {
		fail(c, err)
		return
	}
	defer body.Close()
	timeout := config.Get().Storage.ObjectStorage.UploadTimeout.Duration
	if timeout <= 0 {
		fail(c, errors.New("file upload timeout is not configured"))
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	item, err := s.Upload(ctx, actor, names[0], file.Filename, body)
	if err != nil {
		fail(c, err)
		return
	}
	api.Created(c, item)
}
func Rename(_ context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, ok := fileID(c)
	if !ok {
		return
	}
	s, ok := wired(c)
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if err := c.BindJSON(&input); err != nil {
		fail(c, fileservice.ErrInvalid)
		return
	}
	item, err := s.Rename(actor, id, input.Name)
	if err != nil {
		fail(c, err)
		return
	}
	api.Success(c, item)
}
func Delete(ctx context.Context, c *hertzapp.RequestContext) {
	actor, ok := middleware.AuthenticateRequest(c)
	if !ok {
		return
	}
	id, ok := fileID(c)
	if !ok {
		return
	}
	s, ok := wired(c)
	if !ok {
		return
	}
	if err := s.Delete(ctx, actor, id); err != nil {
		fail(c, err)
		return
	}
	api.Success(c, map[string]bool{"deleted": true})
}
func Resource(ctx context.Context, c *hertzapp.RequestContext) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	s, ok := wired(c)
	if !ok {
		return
	}
	grant, err := s.Resource(ctx, c.Param("token"), c.Query("download") == "1")
	if err != nil {
		fail(c, err)
		return
	}
	c.Redirect(consts.StatusTemporaryRedirect, []byte(grant.URL))
}
