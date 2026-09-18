package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadAlwaysUsesConfigDirectory(t *testing.T) {
	root := t.TempDir()
	writeValidConfig(t, filepath.Join(root, "config"))
	other := t.TempDir()
	writeValidConfig(t, other)
	writeConfigFile(t, other, "app.yaml", validAppYAML("wrong-app"))
	t.Setenv("WT_MEDIA_CONFIG_DIR", other)
	t.Chdir(root)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.App.Name, "wt-media-cloud"; got != want {
		t.Fatalf("App.Name = %q, want %q", got, want)
	}
}

func TestLoadFromDirReadsServerFromAppYAML(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if got, want := cfg.App.Server.HTTPAddr, "127.0.0.1:8080"; got != want {
		t.Fatalf("App.Server.HTTPAddr = %q, want %q", got, want)
	}
	if !cfg.App.Server.SessionCookieSecure {
		t.Fatal("App.Server.SessionCookieSecure = false, want true")
	}
}

func TestLoadFromDirRequiresPrimaryDatabase(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	if err := os.Remove(filepath.Join(dir, "database", "primary.yaml")); err != nil {
		t.Fatal(err)
	}
	writeDatabaseConfig(t, dir, "analytics.yaml", "analytics")

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "primary") {
		t.Fatalf("LoadFromDir() error = %v, want missing primary error", err)
	}
}

func TestLoadFromDirRejectsDuplicateDatabaseNames(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeDatabaseConfig(t, dir, "duplicate.yaml", "primary")

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate database name") {
		t.Fatalf("LoadFromDir() error = %v, want duplicate database name", err)
	}
}

func TestLoadFromDirIgnoresMigrationConfigWhenScanningDatabases(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "database/migration.yaml", "directory: migrations\nunknown_migration_field: true\n")

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if got, want := len(cfg.Databases), 1; got != want {
		t.Fatalf("len(Databases) = %d, want %d", got, want)
	}
}

func TestLoadFromDirLoadsSixLoggerConfigs(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	paths := []string{
		cfg.Loggers.App.Path,
		cfg.Loggers.Access.Path,
		cfg.Loggers.Job.Path,
		cfg.Loggers.External.Path,
		cfg.Loggers.Audit.Path,
		cfg.Loggers.Panic.Path,
	}
	want := []string{
		"logs/app.log",
		"logs/access.log",
		"logs/job.log",
		"logs/external.log",
		"logs/audit.log",
		"logs/panic.log",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("logger paths = %#v, want %#v", paths, want)
	}
}

func TestClientAndCredentialConfigsAreIndependent(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if got, want := cfg.Clients.Douyin.Host, "api.itfaba.com"; got != want {
		t.Fatalf("Douyin client host = %q, want %q", got, want)
	}
	if got, want := cfg.Credentials.Douyin.APIKey, "douyin-key"; got != want {
		t.Fatalf("Douyin API key = %q, want %q", got, want)
	}
	if got, want := cfg.Credentials.Agent.AuthToken, "agent-token"; got != want {
		t.Fatalf("Agent auth token = %q, want %q", got, want)
	}

	clientType := reflect.TypeOf(ClientConfig{})
	for _, forbidden := range []string{"APIKey", "Cookie", "AuthToken", "Headers"} {
		if _, found := clientType.FieldByName(forbidden); found {
			t.Fatalf("ClientConfig unexpectedly contains credential field %q", forbidden)
		}
	}
}

func TestSchedulerDurationsAndBatchSizeAreValidated(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "scheduler/scheduler.yaml", "proxy_expiry_interval: 6h\ndiscovery_interval: 1m\nworker_interval: 5s\nworker_batch_size: 0\n")

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "worker_batch_size") {
		t.Fatalf("LoadFromDir() error = %v, want worker_batch_size validation error", err)
	}
}

func TestInitializePublishesValidatedReadOnlyConfig(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	root := t.TempDir()
	writeValidConfig(t, filepath.Join(root, "config"))
	t.Chdir(root)

	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	first := Get()
	first.Databases[0].Name = "changed"
	first.Credentials.Douyin.Headers["User-Agent"] = "changed"
	second := Get()
	if got, want := second.Databases[0].Name, "primary"; got != want {
		t.Fatalf("published database name = %q, want %q", got, want)
	}
	if got, want := second.Credentials.Douyin.Headers["User-Agent"], "WT-Media-Cloud/1"; got != want {
		t.Fatalf("published header = %q, want %q", got, want)
	}
}

func TestGetPanicsBeforeInitialize(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("Get() did not panic before Initialize")
		}
	}()
	_ = Get()
}

func TestLoadFromDirRejectsUnknownYAMLFieldWithPath(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "app.yaml", validAppYAML("wt-media-cloud")+"unknown_field: true\n")

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("LoadFromDir() error = nil, want unknown field error")
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "app.yaml")) || !strings.Contains(err.Error(), "unknown_field") {
		t.Fatalf("LoadFromDir() error = %v, want file path and unknown field", err)
	}
}

func TestLoadFromDirRejectsInvalidDurationWithPath(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "clients/agent.yaml", "name: agent\nscheme: http\nhost: 127.0.0.1\nport: 8765\ntimeout: eventually\nretry:\n  attempts: 2\n  interval: 300ms\n")

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("LoadFromDir() error = nil, want invalid duration error")
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "clients", "agent.yaml")) || !strings.Contains(err.Error(), "eventually") {
		t.Fatalf("LoadFromDir() error = %v, want file path and invalid duration", err)
	}
}

func TestDurationUsesTimeParseDuration(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if got, want := cfg.Clients.Agent.Timeout.Duration, 7*time.Second; got != want {
		t.Fatalf("Agent timeout = %s, want %s", got, want)
	}
	if got, want := cfg.Scheduler.DiscoveryInterval.Duration, time.Minute; got != want {
		t.Fatalf("Discovery interval = %s, want %s", got, want)
	}
}

func TestConfigValidateRejectsPartialInitialAdmin(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "app.yaml", "name: wt-media-cloud\nserver:\n  http_addr: 127.0.0.1:8080\n  session_cookie_secure: true\ninitial_admin:\n  username: admin\n")

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "initial_admin") {
		t.Fatalf("LoadFromDir() error = %v, want partial initial_admin error", err)
	}
}

func writeValidConfig(t *testing.T, root string) {
	t.Helper()
	writeConfigFile(t, root, "app.yaml", validAppYAML("wt-media-cloud"))
	writeDatabaseConfig(t, root, "primary.yaml", "primary")
	writeConfigFile(t, root, "cache/redis.yaml", "url: \"\"\n")
	for _, category := range []string{"app", "access", "job", "external", "audit", "panic"} {
		writeConfigFile(t, root, "logger/"+category+".yaml", "path: logs/"+category+".log\nlevel: info\nformat: json\n")
	}
	writeConfigFile(t, root, "clients/agent.yaml", "name: agent\nscheme: http\nhost: 127.0.0.1\nport: 8765\ntimeout: 7s\nretry:\n  attempts: 2\n  interval: 300ms\n")
	writeConfigFile(t, root, "clients/platforms/douyin.yaml", "name: douyin\nscheme: https\nhost: api.itfaba.com\nport: 443\ntimeout: 30s\nretry:\n  attempts: 2\n  interval: 300ms\n")
	writeConfigFile(t, root, "credentials/agent.yaml", "auth_token: agent-token\n")
	writeConfigFile(t, root, "credentials/douyin.yaml", "api_key: douyin-key\ncookie: douyin-cookie\nheaders:\n  User-Agent: WT-Media-Cloud/1\n")
	writeConfigFile(t, root, "scheduler/scheduler.yaml", "proxy_expiry_interval: 6h\ndiscovery_interval: 1m\nworker_interval: 5s\nworker_batch_size: 10\n")
	writeConfigFile(t, root, "observability/health.yaml", "health_path: /api/v1/health\n")
}

func validAppYAML(name string) string {
	return "name: " + name + "\nserver:\n  http_addr: 127.0.0.1:8080\n  session_cookie_secure: true\ninitial_admin:\n  username: \"\"\n  password: \"\"\n"
}

func writeDatabaseConfig(t *testing.T, root, fileName, name string) {
	t.Helper()
	writeConfigFile(t, root, "database/"+fileName, "name: "+name+"\nhost: 127.0.0.1\nport: 3306\ndatabase: wt_media\nusername: root\npassword: secret\ncharset: utf8mb4\nparse_time: true\nlocation: Local\npool:\n  max_idle: 10\n  max_open: 50\n  max_lifetime: 30m\n")
}

func writeConfigFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}
