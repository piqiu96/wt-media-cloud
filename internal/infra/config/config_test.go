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

func TestLoadIdentityConfiguration(t *testing.T) {
	t.Setenv("WT_MEDIA_INITIAL_ADMIN_USERNAME", "admin")
	t.Setenv("WT_MEDIA_INITIAL_ADMIN_PASSWORD", "initial-secret")
	t.Setenv("WT_MEDIA_SESSION_COOKIE_SECURE", "false")

	cfg := Load()
	if cfg.InitialAdminUsername != "admin" || cfg.InitialAdminPassword != "initial-secret" {
		t.Fatalf("identity bootstrap config = %+v", cfg)
	}
	if cfg.SessionCookieSecure {
		t.Fatalf("SessionCookieSecure = true, want false")
	}
}
