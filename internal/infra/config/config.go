package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr                  string
	MySQLDSN                  string
	InitialTechnicianUsername string
	InitialTechnicianPassword string
	SessionCookieSecure       bool
}

func Load() Config {
	return Config{
		HTTPAddr:                  env("WT_MEDIA_CLOUD_HTTP_ADDR", ":8080"),
		MySQLDSN:                  os.Getenv("WT_MEDIA_MYSQL_DSN"),
		InitialTechnicianUsername: os.Getenv("WT_MEDIA_INITIAL_TECHNICIAN_USERNAME"),
		InitialTechnicianPassword: os.Getenv("WT_MEDIA_INITIAL_TECHNICIAN_PASSWORD"),
		SessionCookieSecure:       envBool("WT_MEDIA_SESSION_COOKIE_SECURE", true),
	}
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
