package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("WT_MEDIA_CLOUD_HTTP_ADDR", "")
	t.Setenv("WT_MEDIA_MYSQL_DSN", "")

	cfg := Load()
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.MySQLDSN != "" {
		t.Fatalf("MySQLDSN = %q, want empty", cfg.MySQLDSN)
	}
}

func TestLoadOverridesHTTPAddr(t *testing.T) {
	t.Setenv("WT_MEDIA_CLOUD_HTTP_ADDR", "127.0.0.1:18080")

	cfg := Load()
	if cfg.HTTPAddr != "127.0.0.1:18080" {
		t.Fatalf("HTTPAddr = %q, want override", cfg.HTTPAddr)
	}
}
