package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type Config struct {
	HTTPAddr             string
	MySQLDSN             string
	InitialAdminUsername string
	InitialAdminPassword string
	SessionCookieSecure  bool
	LogLevel             string
}

func Load() Config {
	cfg := Config{
		HTTPAddr:             env("WT_MEDIA_CLOUD_HTTP_ADDR", ":8080"),
		MySQLDSN:             os.Getenv("WT_MEDIA_MYSQL_DSN"),
		InitialAdminUsername: envFirst("WT_MEDIA_INITIAL_ADMIN_USERNAME", "WT_MEDIA_INITIAL_TECHNICIAN_USERNAME"),
		InitialAdminPassword: envFirst("WT_MEDIA_INITIAL_ADMIN_PASSWORD", "WT_MEDIA_INITIAL_TECHNICIAN_PASSWORD"),
		SessionCookieSecure:  envBool("WT_MEDIA_SESSION_COOKIE_SECURE", true),
		LogLevel:             env("WT_MEDIA_LOG_LEVEL", "info"),
	}

	// Apply log level.
	switch cfg.LogLevel {
	case "debug":
		hlog.SetLevel(hlog.LevelDebug)
	case "info":
		hlog.SetLevel(hlog.LevelInfo)
	case "warn":
		hlog.SetLevel(hlog.LevelWarn)
	case "error":
		hlog.SetLevel(hlog.LevelError)
	default:
		hlog.SetLevel(hlog.LevelInfo)
	}

	return cfg
}

func (c Config) Validate() error {
	if c.MySQLDSN == "" && (c.InitialAdminUsername != "" || c.InitialAdminPassword != "") {
		return fmt.Errorf("initial admin requires WT_MEDIA_MYSQL_DSN")
	}
	return nil
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
