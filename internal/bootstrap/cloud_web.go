package bootstrap

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const cloudWebEntry = "index.cloud.html"

func registerCloudWeb(engine *server.Hertz, webRoot string) error {
	root, err := filepath.Abs(webRoot)
	if err != nil {
		return fmt.Errorf("resolve Cloud Web root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve Cloud Web root symlinks: %w", err)
	}
	entry := filepath.Join(root, cloudWebEntry)
	entryInfo, err := os.Stat(entry)
	if err != nil {
		return fmt.Errorf("Cloud Web entry %s: %w", entry, err)
	}
	if !entryInfo.Mode().IsRegular() {
		return fmt.Errorf("Cloud Web entry is not a regular file: %s", entry)
	}

	engine.NoRoute(func(_ context.Context, c *hertzapp.RequestContext) {
		method := string(c.Method())
		if method != consts.MethodGet && method != consts.MethodHead {
			c.Status(consts.StatusNotFound)
			return
		}

		requestPath, ok := safeCloudWebPath(string(c.Path()))
		if !ok || reservedCloudPath(requestPath) {
			c.Status(consts.StatusNotFound)
			return
		}

		candidate := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(requestPath, "/")))
		if resolved, found := cloudWebFile(root, candidate); found {
			c.File(resolved)
			return
		}
		c.File(entry)
	})
	return nil
}

func cloudWebFile(root, candidate string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	info, err := os.Stat(resolved)
	return resolved, err == nil && info.Mode().IsRegular()
}

func safeCloudWebPath(raw string) (string, bool) {
	decoded, err := url.PathUnescape(raw)
	if err != nil || strings.IndexByte(decoded, 0) >= 0 {
		return "", false
	}
	for _, segment := range strings.Split(strings.ReplaceAll(decoded, "\\", "/"), "/") {
		if segment == ".." {
			return "", false
		}
	}
	return path.Clean("/" + decoded), true
}

func reservedCloudPath(requestPath string) bool {
	return requestPath == "/api" || strings.HasPrefix(requestPath, "/api/") ||
		requestPath == "/healthz" || strings.HasPrefix(requestPath, "/healthz/")
}
