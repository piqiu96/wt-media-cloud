package middleware

import (
	"context"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func LocalDesktopCORS() hertzapp.HandlerFunc {
	return func(ctx context.Context, c *hertzapp.RequestContext) {
		origin := string(c.Request.Header.Peek("Origin"))
		if IsAllowedLocalDesktopOrigin(origin) {
			c.Response.Header.Set("Access-Control-Allow-Origin", origin)
			c.Response.Header.Set("Access-Control-Allow-Credentials", "true")
			c.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Token")
			c.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Response.Header.Set("Vary", "Origin")
		}
		if string(c.Method()) == consts.MethodOptions {
			c.SetStatusCode(consts.StatusNoContent)
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}

func IsAllowedLocalDesktopOrigin(origin string) bool {
	switch strings.TrimSpace(origin) {
	case "http://tauri.localhost", "https://tauri.localhost", "tauri://localhost", "http://127.0.0.1:5174", "http://localhost:5174":
		return true
	default:
		return false
	}
}
