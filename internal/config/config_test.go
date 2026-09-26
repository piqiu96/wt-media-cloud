package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
)

func TestLoadAlwaysUsesConfigDirectory(t *testing.T) {
	root := t.TempDir()
	writeValidConfig(t, filepath.Join(root, "config"))
	other := t.TempDir()
	writeValidConfig(t, other)
	writeConfigFile(t, other, "app.yaml", validAppYAMLIgnored("wrong-app"))
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

func TestLoadFromDirReadsServerFromAppTOML(t *testing.T) {
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

func TestLoadFromDirAcceptsReleaseConfigTree(t *testing.T) {
	if _, err := LoadFromDir("../../config_online"); err != nil {
		t.Fatalf("LoadFromDir(config_online) error = %v", err)
	}
}

func TestLoadFromDirRejectsNonTOMLCloudConfig(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "database/extra.yaml", "name: extra\n")

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "Cloud config must use TOML") {
		t.Fatalf("LoadFromDir() error = %v, want non-TOML rejection", err)
	}
}

func TestLoadFromDirRequiresPrimaryDatabase(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	if err := os.Remove(filepath.Join(dir, "database", "primary.toml")); err != nil {
		t.Fatal(err)
	}
	writeDatabaseConfig(t, dir, "analytics.toml", "analytics")

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "primary") {
		t.Fatalf("LoadFromDir() error = %v, want missing primary error", err)
	}
}

func TestLoadFromDirRejectsDuplicateDatabaseNames(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeDatabaseConfig(t, dir, "duplicate.toml", "primary")

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate database name") {
		t.Fatalf("LoadFromDir() error = %v, want duplicate database name", err)
	}
}

func TestLoadFromDirIgnoresMigrationConfigWhenScanningDatabases(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "database/migration.toml", "directory = \"migrations\"\nunknown_migration_field = true\n")

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if got, want := len(cfg.Databases), 1; got != want {
		t.Fatalf("len(Databases) = %d, want %d", got, want)
	}
}

func TestLoadFromDirReadsObjectStorage(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	storage := cfg.Storage.ObjectStorage
	if got, want := storage.Endpoint, "127.0.0.1:9000"; got != want {
		t.Fatalf("ObjectStorage.Endpoint = %q, want %q", got, want)
	}
	if got, want := storage.Bucket, "wt-media"; got != want {
		t.Fatalf("ObjectStorage.Bucket = %q, want %q", got, want)
	}
	if got, want := storage.Prefix, "test/"; got != want {
		t.Fatalf("ObjectStorage.Prefix = %q, want %q", got, want)
	}
	if got, want := storage.PresignTTL.Duration, 15*time.Minute; got != want {
		t.Fatalf("ObjectStorage.PresignTTL = %s, want %s", got, want)
	}
	if storage.UseSSL {
		t.Fatal("ObjectStorage.UseSSL = true, want false")
	}
}

// The credential file is absent from every tree this repository ships, and that
// has to stay a loadable state: it is what lets a checkout build and run its
// tests without a secret. Asserted rather than assumed, because making the file
// required would break only the trees that have no secret — which includes the
// release tree as it is committed.
func TestLoadFromDirLoadsWithoutObjectStorageCredentials(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	if _, err := os.Stat(filepath.Join(dir, "credentials", "object_storage.toml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the valid tree unexpectedly has an object storage credential: %v", err)
	}

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v, want a tree without credentials to load", err)
	}
	if got := cfg.Credentials.ObjectStorage; got.AccessKey != "" || got.SecretKey != "" {
		t.Fatalf("ObjectStorage credentials = %+v, want both empty", got)
	}
}

func TestLoadFromDirReadsObjectStorageCredentials(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "credentials/object_storage.toml", "access_key = \"ak\"\nsecret_key = \"sk\"\n")

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if got, want := cfg.Credentials.ObjectStorage.AccessKey, "ak"; got != want {
		t.Fatalf("AccessKey = %q, want %q", got, want)
	}
	if got, want := cfg.Credentials.ObjectStorage.SecretKey, "sk"; got != want {
		t.Fatalf("SecretKey = %q, want %q", got, want)
	}
}

// Half a pair is one of the two shapes a person produces while editing, and it
// must not reach the first request as an authentication failure. Both arms are
// exercised because a check written as "either is empty" would pass the arm that
// leaves `secret_key` behind.
func TestValidateRejectsHalfAnObjectStorageCredentialPair(t *testing.T) {
	for _, testCase := range []struct {
		name string
		toml string
	}{
		{"access key only", "access_key = \"ak\"\nsecret_key = \"\"\n"},
		{"secret key only", "access_key = \"\"\nsecret_key = \"sk\"\n"},
	} {
		dir := t.TempDir()
		writeValidConfig(t, dir)
		writeConfigFile(t, dir, "credentials/object_storage.toml", testCase.toml)

		_, err := LoadFromDir(dir)
		if err == nil || !strings.Contains(err.Error(), "access_key and secret_key") {
			t.Fatalf("%s: LoadFromDir() error = %v, want a paired-credential error", testCase.name, err)
		}
	}
}

func TestLoadFromDirRequiresObjectStorage(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	if err := os.Remove(filepath.Join(dir, "storage", "object_storage.toml")); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "object_storage.toml") {
		t.Fatalf("LoadFromDir() error = %v, want missing object storage error", err)
	}
}

// Each arm deletes one required value, which is what a person does while editing.
// They are refused at load rather than at the first call because the failures they
// would otherwise produce are unrecognisable: an empty bucket reaches the client
// library as a request against whatever the endpoint treats as the default bucket,
// and a zero TTL signs URLs that are already expired.
func TestValidateRejectsAnEmptyObjectStorageField(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		file       string
		wantErrsub string
	}{
		{
			"bucket",
			"bucket = \"\"\nendpoint = \"127.0.0.1:9000\"\nprefix = \"\"\npresign_ttl = \"15m\"\nregion = \"\"\nuse_ssl = false\n",
			"bucket is required",
		},
		{
			"endpoint",
			"bucket = \"wt-media\"\nendpoint = \"\"\nprefix = \"\"\npresign_ttl = \"15m\"\nregion = \"\"\nuse_ssl = false\n",
			"endpoint is required",
		},
		{
			"presign_ttl",
			"bucket = \"wt-media\"\nendpoint = \"127.0.0.1:9000\"\nprefix = \"\"\npresign_ttl = \"0s\"\nregion = \"\"\nuse_ssl = false\n",
			"presign_ttl must be greater than zero",
		},
	} {
		dir := t.TempDir()
		writeValidConfig(t, dir)
		writeConfigFile(t, dir, "storage/object_storage.toml", testCase.file)

		_, err := LoadFromDir(dir)
		if err == nil || !strings.Contains(err.Error(), testCase.wantErrsub) {
			t.Fatalf("%s: LoadFromDir() error = %v, want %q", testCase.name, err, testCase.wantErrsub)
		}
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
	douyin, ok := httpClientByName(cfg, "douyin")
	if !ok || douyin.Endpoint.Origin() != "https://api.itfaba.com:443" {
		t.Fatalf("Douyin client = %+v, found=%v", douyin, ok)
	}
	if got, want := cfg.Credentials.Douyin.APIKey, "douyin-key"; got != want {
		t.Fatalf("Douyin API key = %q, want %q", got, want)
	}
	if got, want := cfg.Credentials.Agent.AuthToken, "agent-token"; got != want {
		t.Fatalf("Agent auth token = %q, want %q", got, want)
	}

	clientType := reflect.TypeOf(httpclient.Config{})
	for _, forbidden := range []string{"APIKey", "Cookie", "AuthToken", "Headers"} {
		if _, found := clientType.FieldByName(forbidden); found {
			t.Fatalf("ClientConfig unexpectedly contains credential field %q", forbidden)
		}
	}
}

func TestSchedulerDurationsAndBatchSizeAreValidated(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "scheduler/scheduler.toml", "proxy_expiry_interval = \"6h\"\ndiscovery_interval = \"1m\"\nworker_interval = \"5s\"\nworker_batch_size = 0\n")

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

func TestLoadFromDirRejectsUnknownTOMLFieldWithPath(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "app.toml", validAppTOML("wt-media-cloud")+"unknown_field = true\n")

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("LoadFromDir() error = nil, want unknown field error")
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "app.toml")) || !strings.Contains(err.Error(), "unknown fields") {
		t.Fatalf("LoadFromDir() error = %v, want file path and unknown field", err)
	}
}

func TestLoadFromDirRejectsInvalidDurationWithPath(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "clients/http/agent.toml", "name = \"agent\"\ntimeout = \"eventually\"\n\n[endpoint]\nscheme = \"http\"\nhost = \"127.0.0.1\"\nport = 8765\n\n[retry]\nattempts = 2\n")

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("LoadFromDir() error = nil, want invalid duration error")
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "clients", "http", "agent.toml")) || !strings.Contains(err.Error(), "eventually") {
		t.Fatalf("LoadFromDir() error = %v, want file path and invalid duration", err)
	}
}

func TestHTTPClientNameComesFromFileContent(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	if err := os.Remove(filepath.Join(dir, "clients", "http", "agent.toml")); err != nil {
		t.Fatal(err)
	}
	writeConfigFile(t, dir, "clients/http/alias.toml", "name = \"agent\"\ntimeout = \"7s\"\n\n[endpoint]\nscheme = \"http\"\nhost = \"127.0.0.1\"\nport = 8765\n\n[connection]\ndial_timeout = \"1s\"\n\n[retry]\nattempts = 2\n")

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if client, ok := httpClientByName(cfg, "agent"); !ok || client.Endpoint.Host != "127.0.0.1" {
		t.Fatalf("agent client = %+v, found=%v", client, ok)
	}
}

func TestDurationUsesTimeParseDuration(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	agent, _ := httpClientByName(cfg, "agent")
	if got, want := agent.Timeout.Duration, 7*time.Second; got != want {
		t.Fatalf("Agent timeout = %s, want %s", got, want)
	}
	if got, want := cfg.Scheduler.DiscoveryInterval.Duration, time.Minute; got != want {
		t.Fatalf("Discovery interval = %s, want %s", got, want)
	}
}

func TestConfigValidateRejectsPartialInitialAdmin(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "app.toml", validAppTOML("wt-media-cloud")+"[initial_admin]\nusername = \"admin\"\npassword = \"\"\n")

	_, err := LoadFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "initial_admin") {
		t.Fatalf("LoadFromDir() error = %v, want partial initial_admin error", err)
	}
}

func writeValidConfig(t *testing.T, root string) {
	t.Helper()
	writeConfigFile(t, root, "app.toml", validAppTOML("wt-media-cloud"))
	writeDatabaseConfig(t, root, "primary.toml", "primary")
	for _, category := range []string{"app", "access", "job", "external", "audit", "panic"} {
		writeConfigFile(t, root, "logger/"+category+".toml", "path = \"logs/"+category+".log\"\nlevel = \"info\"\nformat = \"json\"\n\n[rotation]\nmax_size = 500\nmax_age = 30\nmax_backups = 10\ncompress = true\nlocal_time = true\n")
	}
	writeHTTPClient(t, root, "agent", "http", "127.0.0.1", 8765, "7s")
	writeHTTPClient(t, root, "douyin", "https", "api.itfaba.com", 443, "30s")
	writeConfigFile(t, root, "credentials/agent.toml", "auth_token = \"agent-token\"\n")
	writeConfigFile(t, root, "credentials/douyin.toml", "api_key = \"douyin-key\"\ncookie = \"douyin-cookie\"\n\n[headers]\n\"User-Agent\" = \"WT-Media-Cloud/1\"\n")
	writeConfigFile(t, root, "scheduler/scheduler.toml", "proxy_expiry_interval = \"6h\"\ndiscovery_interval = \"1m\"\nworker_interval = \"5s\"\nworker_batch_size = 10\n")
	writeConfigFile(t, root, "storage/object_storage.toml", validObjectStorageTOML())
}

// validObjectStorageTOML carries no credential: that file is optional, and the
// tree every other test loads is the one without it.
func validObjectStorageTOML() string {
	return "bucket = \"wt-media\"\nendpoint = \"127.0.0.1:9000\"\nprefix = \"test/\"\npresign_ttl = \"15m\"\nregion = \"\"\nuse_ssl = false\n"
}

func writeHTTPClient(t *testing.T, root, name, scheme, host string, port int, timeout string) {
	t.Helper()
	writeConfigFile(t, root, "clients/http/"+name+".toml", "name = "+strconv.Quote(name)+"\ntimeout = "+strconv.Quote(timeout)+"\n\n[endpoint]\nscheme = "+strconv.Quote(scheme)+"\nhost = "+strconv.Quote(host)+"\nport = "+strconv.Itoa(port)+"\n\n[connection]\ndial_timeout = \"1s\"\n\n[retry]\nattempts = 2\ndelay = \"300ms\"\nmax_delay = \"2s\"\npolicy = \"fixed\"\n")
}

func httpClientByName(cfg Config, name string) (httpclient.Config, bool) {
	for _, client := range cfg.Clients.HTTP {
		if client.Name == name {
			return client, true
		}
	}
	return httpclient.Config{}, false
}

func validAppTOML(name string) string {
	return "name = " + strconv.Quote(name) + "\n\n[server]\nhttp_addr = \"127.0.0.1:8080\"\nsession_cookie_secure = true\n\n[initial_admin]\nusername = \"\"\npassword = \"\"\n"
}

func validAppYAMLIgnored(name string) string {
	return "name: " + name + "\nserver:\n  http_addr: 127.0.0.1:1\n  session_cookie_secure: true\ninitial_admin:\n  username: \"\"\n  password: \"\"\n"
}

func writeDatabaseConfig(t *testing.T, root, fileName, name string) {
	t.Helper()
	writeConfigFile(t, root, "database/"+fileName, "name = "+strconv.Quote(name)+"\nhost = \"127.0.0.1\"\nport = 3306\ndatabase = \"wt_media\"\nusername = \"root\"\npassword = \"secret\"\ncharset = \"utf8mb4\"\nparse_time = true\nlocation = \"Local\"\n\n[pool]\nmax_idle = 10\nmax_open = 50\nmax_lifetime = \"30m\"\n")
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
