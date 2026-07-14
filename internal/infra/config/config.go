package config

import (
	"os"
)

type Config struct {
	HTTPAddr string
	MySQLDSN string
}

func Load() Config {
	return Config{
		HTTPAddr: env("WT_MEDIA_CLOUD_HTTP_ADDR", ":8080"),
		MySQLDSN: os.Getenv("WT_MEDIA_MYSQL_DSN"),
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
