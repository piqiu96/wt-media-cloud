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
	initializeRuntimePathsForTest(t)

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

func TestLoadFromDirAcceptsShippedRuntimeConfigTree(t *testing.T) {
	if _, err := LoadFromDir("../../config"); err != nil {
		t.Fatalf("LoadFromDir(config) error = %v", err)
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

func TestLoadFromDirReadsFFmpegToolConfig(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	tools := cfg.Media.FFmpeg
	if got, want := tools.FFmpegPath, "/opt/homebrew/bin/ffmpeg"; got != want {
		t.Fatalf("Media.FFmpeg.FFmpegPath = %q, want %q", got, want)
	}
	if got, want := tools.FFprobePath, "/opt/homebrew/bin/ffprobe"; got != want {
		t.Fatalf("Media.FFmpeg.FFprobePath = %q, want %q", got, want)
	}
	if got, want := tools.Version, "8.1.2"; got != want {
		t.Fatalf("Media.FFmpeg.Version = %q, want %q", got, want)
	}
	if got, want := tools.RequiredEncoders, []string{"libx264", "aac"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Media.FFmpeg.RequiredEncoders = %v, want %v", got, want)
	}
	if tools.FFmpegSHA256 != "" || tools.FFprobeSHA256 != "" {
		t.Fatalf("Media.FFmpeg sha pins = %q/%q, want both empty in the dev tree", tools.FFmpegSHA256, tools.FFprobeSHA256)
	}
}

// A tree without tool configuration must fail to load rather than start a
// compose run that would discover a missing binary mid-flight.
func TestLoadFromDirRequiresFFmpegToolConfig(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	if err := os.Remove(filepath.Join(dir, "media", "ffmpeg.toml")); err != nil {
		t.Fatalf("remove ffmpeg tool config: %v", err)
	}

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("LoadFromDir() = nil error, want a failure when media/ffmpeg.toml is absent")
	}
	if !strings.Contains(err.Error(), "media/ffmpeg.toml") {
		t.Fatalf("LoadFromDir() error = %v, want it to name the missing file", err)
	}
}

func TestValidateRejectsAnEmptyFFmpegToolField(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	writeConfigFile(t, dir, "media/ffmpeg.toml", "ffmpeg_path = \"/opt/homebrew/bin/ffmpeg\"\nffprobe_path = \"/opt/homebrew/bin/ffprobe\"\nversion = \"\"\nrequired_encoders = [\"libx264\"]\n")

	if _, err := LoadFromDir(dir); err == nil {
		t.Fatal("LoadFromDir() = nil error, want a failure for an empty tool version")
	} else if !strings.Contains(err.Error(), "version") {
		t.Fatalf("LoadFromDir() error = %v, want it to name the empty field", err)
	}
}

func TestLoadFromDirReadsComposeWorkerConfig(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	if got, want := cfg.Compose.Worker.WorkspaceRoot, "compose-workspace"; got != want {
		t.Fatalf("Compose.Worker.WorkspaceRoot = %q, want %q", got, want)
	}
}

func TestLoadFromDirRequiresComposeWorkerConfig(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)
	if err := os.Remove(filepath.Join(dir, "scheduler", "compose.toml")); err != nil {
		t.Fatalf("remove compose worker config: %v", err)
	}

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("LoadFromDir() = nil error, want a failure when scheduler/compose.toml is absent")
	}
	if !strings.Contains(err.Error(), "scheduler/compose.toml") {
		t.Fatalf("LoadFromDir() error = %v, want it to name the missing file", err)
	}
}

// M4-C5 T-04: the runtime settings the compose worker loop runs on. LoadFromDir
// must read every knob, and a value that would make the loop misbehave — an
// unbounded worker pool, a heartbeat as long as the lease it must beat — must
// refuse startup instead of degrading (spec §5.2).
func TestLoadFromDirReadsComposeWorkerRuntimeSettings(t *testing.T) {
	dir := t.TempDir()
	writeValidConfig(t, dir)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error = %v", err)
	}
	worker := cfg.Compose.Worker
	if worker.NodeID != 1 {
		t.Fatalf("NodeID = %d, want 1", worker.NodeID)
	}
	if worker.Concurrency != 2 {
		t.Fatalf("Concurrency = %d, want 2", worker.Concurrency)
	}
	for name, got := range map[string]time.Duration{
		"lease":        worker.Lease.Duration,
		"heartbeat":    worker.HeartbeatInterval.Duration,
		"task_timeout": worker.TaskTimeout.Duration,
		"poll":         worker.PollInterval.Duration,
		"recovery":     worker.RecoveryInterval.Duration,
	} {
		if got <= 0 {
			t.Fatalf("%s = %v, want a positive duration", name, got)
		}
	}
	if worker.Lease.Duration != 5*time.Minute {
		t.Fatalf("Lease = %v, want 5m", worker.Lease.Duration)
	}
}

func TestLoadFromDirRefusesInvalidComposeWorkerSettings(t *testing.T) {
	cases := []struct {
		name  string
		toml  string
		fails string
	}{
		{"missing node id", "node_id = 0\n", "node_id"},
		{"unbounded concurrency", "concurrency = 0\n", "concurrency"},
		{"no lease", "lease = \"0s\"\n", "lease"},
		{"heartbeat not shorter than the lease", "heartbeat_interval = \"5m\"\n", "heartbeat_interval"},
		{"no task timeout", "task_timeout = \"0s\"\n", "task_timeout"},
		{"no poll interval", "poll_interval = \"0s\"\n", "poll_interval"},
		{"no recovery interval", "recovery_interval = \"0s\"\n", "recovery_interval"},
		{"no free-space floor", "min_free_bytes = 0\n", "min_free_bytes"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()
			writeValidConfig(t, dir)
			writeConfigFile(t, dir, "scheduler/compose.toml", validComposeWorkerTOML()+testCase.toml)

			_, err := LoadFromDir(dir)
			if err == nil {
				t.Fatalf("LoadFromDir() = nil error, want a failure for %s", testCase.name)
			}
			if !strings.Contains(err.Error(), testCase.fails) {
				t.Fatalf("LoadFromDir() error = %v, want it to name %s", err, testCase.fails)
			}
		})
	}
}

// Relative tool paths and the compose workspace anchor to the Cloud home the
// same way logger paths do, so the tracked tree never embeds a user path and
// the release tree can say "bin/ffmpeg".
func TestLoadAnchorsRelativeToolPathsAndWorkspaceToHome(t *testing.T) {
	root := t.TempDir()
	writeValidConfig(t, filepath.Join(root, "config"))
	writeConfigFile(t, filepath.Join(root, "config"), "media/ffmpeg.toml", "ffmpeg_path = \"bin/ffmpeg\"\nffprobe_path = \"bin/ffprobe\"\nversion = \"8.1.2\"\nrequired_encoders = [\"libx264\", \"aac\"]\n")
	writeConfigFile(t, filepath.Join(root, "config"), "scheduler/compose.toml", strings.ReplaceAll(validComposeWorkerTOML(), "compose-workspace", ".compose"))
	t.Setenv("WT_MEDIA_CONFIG_DIR", filepath.Join(root, "config"))
	t.Chdir(root)
	initializeRuntimePathsForTest(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Media.FFmpeg.FFmpegPath; !filepath.IsAbs(got) || !strings.HasSuffix(got, "bin/ffmpeg") {
		t.Fatalf("Media.FFmpeg.FFmpegPath = %q, want an absolute path ending in bin/ffmpeg", got)
	}
	if got := cfg.Compose.Worker.WorkspaceRoot; !filepath.IsAbs(got) || !strings.HasSuffix(got, ".compose") {
		t.Fatalf("Compose.Worker.WorkspaceRoot = %q, want an absolute path ending in .compose", got)
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
		"app.log",
		"access.log",
		"job.log",
		"external.log",
		"audit.log",
		"panic.log",
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
	writeConfigFile(t, dir, "scheduler/scheduler.toml", "proxy_expiry_interval = \"6h\"\ndiscovery_interval = \"1m\"\ncompose_interval = \"30s\"\nworker_interval = \"5s\"\nworker_batch_size = 0\ntask_recovery_interval = \"30s\"\ntask_recovery_batch_size = 100\n")

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
	resetRuntimePathsForTest(t)

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
		writeConfigFile(t, root, "logger/"+category+".toml", "path = \""+category+".log\"\nlevel = \"info\"\nformat = \"json\"\n\n[rotation]\nmax_size = 500\nmax_age = 30\nmax_backups = 10\ncompress = true\nlocal_time = true\n")
	}
	writeHTTPClient(t, root, "agent", "http", "127.0.0.1", 8765, "7s")
	writeHTTPClient(t, root, "douyin", "https", "api.itfaba.com", 443, "30s")
	writeConfigFile(t, root, "credentials/agent.toml", "auth_token = \"agent-token\"\n")
	writeConfigFile(t, root, "credentials/agent_token.toml", validAgentTokenCredentialTOML())
	writeConfigFile(t, root, "credentials/douyin.toml", "api_key = \"douyin-key\"\ncookie = \"douyin-cookie\"\n\n[headers]\n\"User-Agent\" = \"WT-Media-Cloud/1\"\n")
	writeConfigFile(t, root, "scheduler/scheduler.toml", "proxy_expiry_interval = \"6h\"\ndiscovery_interval = \"1m\"\ncompose_interval = \"30s\"\nworker_interval = \"5s\"\nworker_batch_size = 10\ntask_recovery_interval = \"30s\"\ntask_recovery_batch_size = 100\n")
	writeConfigFile(t, root, "scheduler/compose.toml", validComposeWorkerTOML())
	writeConfigFile(t, root, "media/ffmpeg.toml", "ffmpeg_path = \"/opt/homebrew/bin/ffmpeg\"\nffprobe_path = \"/opt/homebrew/bin/ffprobe\"\nversion = \"8.1.2\"\nrequired_encoders = [\"libx264\", \"aac\"]\n")
	writeConfigFile(t, root, "storage/object_storage.toml", validObjectStorageTOML())
}

// validComposeWorkerTOML gives every §5.2 knob a value: a config tree missing
// one must refuse to start the worker, so the fixture cannot omit them either.
func validComposeWorkerTOML() string {
	return "workspace_root = \"compose-workspace\"\n" +
		"node_id = 1\n" +
		"concurrency = 2\n" +
		"global_concurrency = 2\n" +
		"min_free_bytes = 10737418240\n" +
		"lease = \"5m\"\n" +
		"heartbeat_interval = \"30s\"\n" +
		"task_timeout = \"30m\"\n" +
		"poll_interval = \"2s\"\n" +
		"recovery_interval = \"30s\"\n"
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

func validAgentTokenCredentialTOML() string {
	return "secret = \"test-agent-token-secret-with-at-least-32-characters\"\n"
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
