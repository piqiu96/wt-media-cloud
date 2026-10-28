// Package config loads, validates, and exposes read-only process configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
	pkgconfig "github.com/wt-media/wt-media-cloud/pkg/config"
)

const (
	configDirectory     = "config"
	defaultDatabaseName = "primary"
)

var current struct {
	sync.RWMutex
	config      Config
	initialized bool
}

type Config struct {
	App         AppConfig
	Databases   []DatabaseConfig
	Loggers     LoggerConfigs
	Clients     ClientsConfig
	Credentials CredentialsConfig
	Scheduler   SchedulerConfig
	Storage     StorageConfig
	Media       MediaConfig
	Compose     ComposeConfig
}

type AppConfig struct {
	Name         string             `toml:"name"`
	Server       ServerConfig       `toml:"server"`
	InitialAdmin InitialAdminConfig `toml:"initial_admin"`
	BitBrowser   BitBrowserConfig   `toml:"bitbrowser"`
}

type ServerConfig struct {
	HTTPAddr            string `toml:"http_addr"`
	SessionCookieSecure bool   `toml:"session_cookie_secure"`
}

// BitBrowserConfig 保存 BitBrowser 观测的时效设置。
type BitBrowserConfig struct {
	// ObservationFreshFor 是 Agent 的 observed_at 仍可用于判断本机状态的最大年龄。
	// 零值表示使用包内默认值。
	ObservationFreshFor Duration `toml:"observation_fresh_for"`
}

type InitialAdminConfig struct {
	Username string `toml:"username"`
	Password string `toml:"password"`
}

type DatabaseConfig struct {
	Name      string             `toml:"name"`
	Host      string             `toml:"host"`
	Port      int                `toml:"port"`
	Database  string             `toml:"database"`
	Username  string             `toml:"username"`
	Password  string             `toml:"password"`
	Charset   string             `toml:"charset"`
	ParseTime bool               `toml:"parse_time"`
	Location  string             `toml:"location"`
	Pool      DatabasePoolConfig `toml:"pool"`
}

type DatabasePoolConfig struct {
	MaxIdle     int      `toml:"max_idle"`
	MaxOpen     int      `toml:"max_open"`
	MaxLifetime Duration `toml:"max_lifetime"`
}

type LoggerConfigs struct {
	App      LoggerConfig
	Access   LoggerConfig
	Job      LoggerConfig
	External LoggerConfig
	Audit    LoggerConfig
	Panic    LoggerConfig
}

type LoggerConfig struct {
	Path     string         `toml:"path"`
	Level    string         `toml:"level"`
	Format   string         `toml:"format"`
	Rotation RotationConfig `toml:"rotation"`
}

type RotationConfig struct {
	MaxSize    int  `toml:"max_size"`
	MaxAge     int  `toml:"max_age"`
	MaxBackups int  `toml:"max_backups"`
	Compress   bool `toml:"compress"`
	LocalTime  bool `toml:"local_time"`
}

type ClientsConfig struct {
	HTTP []httpclient.Config
}

type CredentialsConfig struct {
	Agent         AgentCredentialConfig
	AgentToken    AgentTokenCredentialConfig
	Douyin        DouyinCredentialConfig
	ObjectStorage ObjectStorageCredentialConfig
}

// ObjectStorageConfig names the bucket every prepared source object lives in.
type ObjectStorageConfig struct {
	Endpoint   string   `toml:"endpoint"`
	Bucket     string   `toml:"bucket"`
	Region     string   `toml:"region"`
	Prefix     string   `toml:"prefix"`
	UseSSL     bool     `toml:"use_ssl"`
	PresignTTL Duration `toml:"presign_ttl"`
}

// ObjectStorageCredentialConfig is deliberately optional in both directions.
//
// The repository ships no value for it, so `go test ./...` and a release build
// start without object-storage secrets and answer `ErrNotConfigured` until they
// are supplied. See `config/README.md` for where the file is expected and why it
// is not tracked.
type ObjectStorageCredentialConfig struct {
	AccessKey string `toml:"access_key"`
	SecretKey string `toml:"secret_key"`
}

// StorageConfig groups the storage backends the worker writes prepared sources to.
type StorageConfig struct {
	ObjectStorage ObjectStorageConfig
}

// MediaConfig groups the external media tooling Cloud shells out to.
type MediaConfig struct {
	FFmpeg FFmpegConfig
}

// FFmpegConfig names the video-processing binaries the compose worker depends
// on and the facts they must report at startup. Paths may be relative; Load
// anchors them to the Cloud home so the release tree can say `bin/ffmpeg`
// without embedding a deployment path. The sha pins are optional in both
// directions like the object-storage credential: a development tree runs the
// tools its platform installed, a controlled deployment pins the shipped bytes.
type FFmpegConfig struct {
	FFmpegPath       string   `toml:"ffmpeg_path"`
	FFprobePath      string   `toml:"ffprobe_path"`
	Version          string   `toml:"version"`
	RequiredEncoders []string `toml:"required_encoders"`
	FFmpegSHA256     string   `toml:"ffmpeg_sha256"`
	FFprobeSHA256    string   `toml:"ffprobe_sha256"`
}

// ComposeConfig groups the compose worker's operational settings. Lease and
// concurrency settings join here in M4-C5; the workspace root is the one the
// M4-C4 verification runs already need.
type ComposeConfig struct {
	Worker ComposeWorkerConfig
}

type ComposeWorkerConfig struct {
	WorkspaceRoot string `toml:"workspace_root"`
	// NodeID names this worker in compose_tasks.claimed_node_id — the value
	// every conditional lifecycle write is keyed by. A deployment runs one
	// compose worker, so a stable configured number is the identity.
	NodeID            int64 `toml:"node_id"`
	Concurrency       int   `toml:"concurrency"`
	GlobalConcurrency int   `toml:"global_concurrency"`
	// MinFreeBytes is the free-space floor on the workspace volume: below it
	// the worker stops claiming until space recovers.
	MinFreeBytes      int64    `toml:"min_free_bytes"`
	Lease             Duration `toml:"lease"`
	HeartbeatInterval Duration `toml:"heartbeat_interval"`
	TaskTimeout       Duration `toml:"task_timeout"`
	PollInterval      Duration `toml:"poll_interval"`
	RecoveryInterval  Duration `toml:"recovery_interval"`
}

type AgentCredentialConfig struct {
	AuthToken string `toml:"auth_token"`
}

// AgentTokenCredentialConfig holds the HMAC key that signs stateless Local
// Agent tokens. Unlike the outgoing Agent auth token, this secret is required:
// without it the server cannot verify any Agent request, so startup fails
// rather than silently serving an unauthenticated executor surface.
type AgentTokenCredentialConfig struct {
	Secret string `toml:"secret"`
}

type DouyinCredentialConfig struct {
	APIKey  string            `toml:"api_key"`
	Cookie  string            `toml:"cookie"`
	Headers map[string]string `toml:"headers"`
}

type SchedulerConfig struct {
	ProxyExpiryInterval Duration `toml:"proxy_expiry_interval"`
	DiscoveryInterval   Duration `toml:"discovery_interval"`
	ComposeInterval     Duration `toml:"compose_interval"`
	WorkerInterval      Duration `toml:"worker_interval"`
	WorkerBatchSize     int      `toml:"worker_batch_size"`

	SessionCleanupInterval  Duration `toml:"session_cleanup_interval"`
	SessionCleanupRetention Duration `toml:"session_cleanup_retention"`
	SessionCleanupBatchSize int      `toml:"session_cleanup_batch_size"`
}

// Duration is a strict YAML duration parsed with time.ParseDuration.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(value []byte) error {
	text := strings.TrimSpace(string(value))
	parsed, err := time.ParseDuration(text)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", text, err)
	}
	d.Duration = parsed
	return nil
}

// Initialize loads the release's configuration, validates it, and atomically
// publishes a copy.
func Initialize() error {
	MustInitializeRuntimePaths()
	cfg, err := Load()
	if err != nil {
		return err
	}
	current.Lock()
	current.config = clone(cfg)
	current.initialized = true
	current.Unlock()
	return nil
}

// Get returns an isolated copy of the initialized process configuration.
func Get() Config {
	current.RLock()
	defer current.RUnlock()
	if !current.initialized {
		panic("config: Get called before Initialize")
	}
	return clone(current.config)
}

// Load reads the runtime configuration from the initialized release paths and
// anchors relative logger paths to the release log directory.
func Load() (Config, error) {
	paths := GetRuntimePaths()
	cfg, err := LoadFromDir(paths.Config)
	if err != nil {
		return Config{}, fmt.Errorf("load runtime config from %q: %w", paths.Config, err)
	}
	resolveRelativeLoggerPaths(&cfg, paths.Logs)
	resolveRelativeMediaAndComposePaths(&cfg, paths.Home)
	return cfg, nil
}

// resolveRelativeLoggerPaths anchors relative logger paths to the configured
// log directory so logs always land under <home>/logs (or $WT_MEDIA_CLOUD_LOG_PATH)
// regardless of the process working directory.
func resolveRelativeLoggerPaths(cfg *Config, logDir string) {
	for _, logger := range []*LoggerConfig{
		&cfg.Loggers.App,
		&cfg.Loggers.Access,
		&cfg.Loggers.Job,
		&cfg.Loggers.External,
		&cfg.Loggers.Audit,
		&cfg.Loggers.Panic,
	} {
		path := strings.TrimSpace(logger.Path)
		if path == "" || filepath.IsAbs(path) {
			continue
		}
		logger.Path = filepath.Join(logDir, path)
	}
}

// resolveRelativeMediaAndComposePaths anchors relative ffmpeg/ffprobe paths and
// the compose workspace root to the Cloud home, for the same reason logger
// paths anchor to the logs directory: the tracked tree and the release tree
// state locations relative to the installation, and the process working
// directory must never decide where a binary or a workspace lives.
func resolveRelativeMediaAndComposePaths(cfg *Config, home string) {
	if path := strings.TrimSpace(cfg.Media.FFmpeg.FFmpegPath); path != "" && !filepath.IsAbs(path) {
		cfg.Media.FFmpeg.FFmpegPath = filepath.Join(home, path)
	}
	if path := strings.TrimSpace(cfg.Media.FFmpeg.FFprobePath); path != "" && !filepath.IsAbs(path) {
		cfg.Media.FFmpeg.FFprobePath = filepath.Join(home, path)
	}
	if path := strings.TrimSpace(cfg.Compose.Worker.WorkspaceRoot); path != "" && !filepath.IsAbs(path) {
		cfg.Compose.Worker.WorkspaceRoot = filepath.Join(home, path)
	}
}

// LoadFromDir loads and validates a configuration tree rooted at root.
func LoadFromDir(root string) (Config, error) {
	var cfg Config
	if err := requiredTOML(filepath.Join(root, "app.toml"), &cfg.App); err != nil {
		return Config{}, err
	}

	databases, err := loadDatabases(filepath.Join(root, "database"))
	if err != nil {
		return Config{}, err
	}
	cfg.Databases = databases

	loggerFiles := []struct {
		name string
		dst  *LoggerConfig
	}{
		{"app", &cfg.Loggers.App},
		{"access", &cfg.Loggers.Access},
		{"job", &cfg.Loggers.Job},
		{"external", &cfg.Loggers.External},
		{"audit", &cfg.Loggers.Audit},
		{"panic", &cfg.Loggers.Panic},
	}
	for _, file := range loggerFiles {
		if err := requiredTOML(filepath.Join(root, "logger", file.name+".toml"), file.dst); err != nil {
			return Config{}, err
		}
	}
	clients, err := loadHTTPClients(filepath.Join(root, "clients", "http"))
	if err != nil {
		return Config{}, err
	}
	cfg.Clients.HTTP = clients
	if err := optionalTOML(filepath.Join(root, "credentials", "agent.toml"), &cfg.Credentials.Agent); err != nil {
		return Config{}, err
	}
	if err := optionalTOML(filepath.Join(root, "credentials", "agent_token.toml"), &cfg.Credentials.AgentToken); err != nil {
		return Config{}, err
	}
	if err := optionalTOML(filepath.Join(root, "credentials", "douyin.toml"), &cfg.Credentials.Douyin); err != nil {
		return Config{}, err
	}
	if err := requiredTOML(filepath.Join(root, "scheduler", "scheduler.toml"), &cfg.Scheduler); err != nil {
		return Config{}, err
	}
	if err := requiredTOML(filepath.Join(root, "scheduler", "compose.toml"), &cfg.Compose.Worker); err != nil {
		return Config{}, err
	}
	if err := requiredTOML(filepath.Join(root, "media", "ffmpeg.toml"), &cfg.Media.FFmpeg); err != nil {
		return Config{}, err
	}
	if err := requiredTOML(filepath.Join(root, "storage", "object_storage.toml"), &cfg.Storage.ObjectStorage); err != nil {
		return Config{}, err
	}
	// Optional, unlike the two above it: the endpoint and bucket are facts about
	// where objects go, while the credential is a secret the deployment supplies.
	// Requiring the file would make every checkout that has no secret fail to load
	// its configuration, which is the failure mode the value-less tree exists to
	// avoid.
	if err := optionalTOML(filepath.Join(root, "credentials", "object_storage.toml"), &cfg.Credentials.ObjectStorage); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.App.Name) == "" {
		return errors.New("app.name is required")
	}
	if strings.TrimSpace(c.App.Server.HTTPAddr) == "" {
		return errors.New("app.server.http_addr is required")
	}
	if (c.App.InitialAdmin.Username == "") != (c.App.InitialAdmin.Password == "") {
		return errors.New("app.initial_admin username and password must be configured together")
	}
	if len(strings.TrimSpace(c.Credentials.AgentToken.Secret)) < 32 {
		return errors.New("credentials.agent_token.secret must be at least 32 characters")
	}
	if c.App.BitBrowser.ObservationFreshFor.Duration < 0 {
		return errors.New("app.bitbrowser.observation_fresh_for must not be negative")
	}
	if err := validateDatabases(c.Databases); err != nil {
		return err
	}
	for name, logger := range map[string]LoggerConfig{
		"app": c.Loggers.App, "access": c.Loggers.Access, "job": c.Loggers.Job,
		"external": c.Loggers.External, "audit": c.Loggers.Audit, "panic": c.Loggers.Panic,
	} {
		if strings.TrimSpace(logger.Path) == "" {
			return fmt.Errorf("logger.%s.path is required", name)
		}
		if logger.Format != "json" && logger.Format != "text" && logger.Format != "console" {
			return fmt.Errorf("logger.%s.format must be json, console, or text", name)
		}
		if logger.Rotation.MaxSize <= 0 {
			return fmt.Errorf("logger.%s.rotation.max_size must be greater than zero", name)
		}
		if logger.Rotation.MaxAge < 0 || logger.Rotation.MaxBackups < 0 {
			return fmt.Errorf("logger.%s.rotation.max_age and max_backups must not be negative", name)
		}
	}
	if err := validateHTTPClients(c.Clients.HTTP); err != nil {
		return err
	}
	if c.Scheduler.ProxyExpiryInterval.Duration <= 0 {
		return errors.New("scheduler.proxy_expiry_interval must be greater than zero")
	}
	if c.Scheduler.DiscoveryInterval.Duration <= 0 {
		return errors.New("scheduler.discovery_interval must be greater than zero")
	}
	if c.Scheduler.ComposeInterval.Duration <= 0 {
		return errors.New("scheduler.compose_interval must be greater than zero")
	}
	if c.Scheduler.WorkerInterval.Duration <= 0 {
		return errors.New("scheduler.worker_interval must be greater than zero")
	}
	if c.Scheduler.WorkerBatchSize <= 0 {
		return errors.New("scheduler.worker_batch_size must be greater than zero")
	}
	if c.Scheduler.SessionCleanupInterval.Duration <= 0 {
		return errors.New("scheduler.session_cleanup_interval must be greater than zero")
	}
	if c.Scheduler.SessionCleanupRetention.Duration <= 0 {
		return errors.New("scheduler.session_cleanup_retention must be greater than zero")
	}
	if c.Scheduler.SessionCleanupBatchSize <= 0 {
		return errors.New("scheduler.session_cleanup_batch_size must be greater than zero")
	}
	if err := validateObjectStorage(c.Storage.ObjectStorage, c.Credentials.ObjectStorage); err != nil {
		return err
	}
	if err := validateFFmpegTools(c.Media.FFmpeg); err != nil {
		return err
	}
	if err := validateComposeWorker(c.Compose.Worker); err != nil {
		return err
	}
	return nil
}

// validateFFmpegTools requires the tool facts the compose worker cannot run
// without. Unlike the storage credential, these are not secrets and there is no
// tree that ships without them: a deployment that has not decided which ffmpeg
// it runs must fail at startup, not at the first compose run.
func validateFFmpegTools(tools FFmpegConfig) error {
	if strings.TrimSpace(tools.FFmpegPath) == "" {
		return errors.New("media.ffmpeg.ffmpeg_path is required")
	}
	if strings.TrimSpace(tools.FFprobePath) == "" {
		return errors.New("media.ffmpeg.ffprobe_path is required")
	}
	if strings.TrimSpace(tools.Version) == "" {
		return errors.New("media.ffmpeg.version is required")
	}
	if len(tools.RequiredEncoders) == 0 {
		return errors.New("media.ffmpeg.required_encoders must name at least one encoder")
	}
	return nil
}

// validateComposeWorker refuses any setting whose absence would make the loop
// degrade instead of run: an unbounded worker pool, a heartbeat that cannot
// beat the lease it renews, a task without a timeout. Spec §5.2: a config
// missing or illegal values must stop the process at startup.
func validateComposeWorker(worker ComposeWorkerConfig) error {
	if strings.TrimSpace(worker.WorkspaceRoot) == "" {
		return errors.New("scheduler.compose.workspace_root is required")
	}
	if worker.NodeID <= 0 {
		return errors.New("scheduler.compose.node_id must be a positive number")
	}
	if worker.Concurrency <= 0 {
		return errors.New("scheduler.compose.concurrency must be greater than zero")
	}
	if worker.GlobalConcurrency <= 0 || worker.Concurrency > worker.GlobalConcurrency {
		return errors.New("scheduler.compose.global_concurrency must be positive and at least concurrency")
	}
	if worker.MinFreeBytes <= 0 {
		return errors.New("scheduler.compose.min_free_bytes must be greater than zero")
	}
	if worker.Lease.Duration <= 0 {
		return errors.New("scheduler.compose.lease must be greater than zero")
	}
	if worker.HeartbeatInterval.Duration <= 0 || worker.HeartbeatInterval.Duration >= worker.Lease.Duration {
		return errors.New("scheduler.compose.heartbeat_interval must be positive and shorter than lease")
	}
	if worker.TaskTimeout.Duration <= 0 {
		return errors.New("scheduler.compose.task_timeout must be greater than zero")
	}
	if worker.PollInterval.Duration <= 0 {
		return errors.New("scheduler.compose.poll_interval must be greater than zero")
	}
	if worker.RecoveryInterval.Duration <= 0 {
		return errors.New("scheduler.compose.recovery_interval must be greater than zero")
	}
	return nil
}

// validateObjectStorage checks where objects go, and only that the credential
// pair is whole.
//
// It does **not** require the credential to be present. A tree with no secret is
// a legitimate state — the repository ships one — and the storage package reports
// `ErrNotConfigured` for it rather than refusing to start. Half a pair is a
// different thing: that is a file someone edited and did not finish, and it would
// otherwise surface as an authentication failure at the first request instead of
// at startup, which is much harder to read.
func validateObjectStorage(storage ObjectStorageConfig, credentials ObjectStorageCredentialConfig) error {
	if strings.TrimSpace(storage.Endpoint) == "" {
		return errors.New("storage.object_storage.endpoint is required")
	}
	if strings.TrimSpace(storage.Bucket) == "" {
		return errors.New("storage.object_storage.bucket is required")
	}
	if storage.PresignTTL.Duration <= 0 {
		return errors.New("storage.object_storage.presign_ttl must be greater than zero")
	}
	accessKey := strings.TrimSpace(credentials.AccessKey)
	secretKey := strings.TrimSpace(credentials.SecretKey)
	if (accessKey == "") != (secretKey == "") {
		return errors.New("credentials.object_storage access_key and secret_key must be configured together")
	}
	return nil
}

func validateDatabases(databases []DatabaseConfig) error {
	foundPrimary := false
	seen := make(map[string]struct{}, len(databases))
	for _, database := range databases {
		name := strings.TrimSpace(database.Name)
		if name == "" {
			return errors.New("database.name is required")
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate database name %q", name)
		}
		seen[name] = struct{}{}
		if name == defaultDatabaseName {
			foundPrimary = true
		}
		if database.Host == "" || database.Port <= 0 || database.Database == "" || database.Username == "" {
			return fmt.Errorf("database %q requires host, port, database, and username", name)
		}
		if database.Charset == "" || database.Location == "" {
			return fmt.Errorf("database %q requires charset and location", name)
		}
		if database.Pool.MaxIdle < 0 || database.Pool.MaxOpen <= 0 || database.Pool.MaxLifetime.Duration <= 0 {
			return fmt.Errorf("database %q pool settings are invalid", name)
		}
	}
	if !foundPrimary {
		return fmt.Errorf("database %q is required", defaultDatabaseName)
	}
	return nil
}

func validateHTTPClients(clients []httpclient.Config) error {
	seen := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		if _, duplicate := seen[client.Name]; duplicate {
			return fmt.Errorf("duplicate http client name %q", client.Name)
		}
		seen[client.Name] = struct{}{}
		if err := httpclient.Validate(client); err != nil {
			return fmt.Errorf("http client %q: %w", client.Name, err)
		}
	}
	for _, name := range []string{"agent", "douyin"} {
		if _, exists := seen[name]; !exists {
			return fmt.Errorf("http client %q is required", name)
		}
	}
	return nil
}

func loadHTTPClients(directory string) ([]httpclient.Config, error) {
	documents, err := pkgconfig.LoadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("http clients directory %s does not exist", directory)
	}
	if err != nil {
		return nil, err
	}
	clients := make([]httpclient.Config, 0, len(documents))
	for _, document := range documents {
		if document.Format != pkgconfig.FormatTOML {
			return nil, fmt.Errorf("Cloud config must use TOML: %s", document.Path)
		}
		var transport httpclient.Config
		if err := decodeFile(document.Path, document, &transport); err != nil {
			return nil, err
		}
		if err := httpclient.Validate(transport); err != nil {
			return nil, fmt.Errorf("http client %s: %w", document.Name, err)
		}
		clients = append(clients, transport)
	}
	return clients, nil
}

func loadDatabases(directory string) ([]DatabaseConfig, error) {
	documents, err := pkgconfig.LoadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("database %q is required: directory %s does not exist", defaultDatabaseName, directory)
	}
	if err != nil {
		return nil, err
	}
	databases := make([]DatabaseConfig, 0, len(documents))
	seen := make(map[string]string)
	for _, document := range documents {
		if document.Name == "migration" {
			continue
		}
		if document.Format != pkgconfig.FormatTOML {
			return nil, fmt.Errorf("Cloud config must use TOML: %s", document.Path)
		}
		var database DatabaseConfig
		if err := document.Decode(&database); err != nil {
			return nil, fmt.Errorf("decode config %s: %w", document.Path, err)
		}
		if previous, duplicate := seen[database.Name]; duplicate {
			return nil, fmt.Errorf("duplicate database name %q in %s and %s", database.Name, previous, document.Path)
		}
		seen[database.Name] = document.Path
		databases = append(databases, database)
	}
	return databases, nil
}

func requiredTOML(path string, dst any) error {
	document, err := loadCloudFile(path)
	if err != nil {
		return err
	}
	if err := decodeFile(path, document, dst); err != nil {
		return err
	}
	return nil
}

func optionalTOML(path string, dst any) error {
	document, err := loadCloudFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := decodeFile(path, document, dst); err != nil {
		return err
	}
	return nil
}

func decodeFile(path string, document pkgconfig.File, dst any) error {
	err := document.Decode(dst)
	if err == nil {
		return nil
	}
	var missing *toml.StrictMissingError
	if errors.As(err, &missing) {
		return fmt.Errorf("decode config %s: unknown fields: %w", path, err)
	}
	return fmt.Errorf("decode config %s: %w", path, err)
}

func loadCloudFile(path string) (pkgconfig.File, error) {
	document, err := pkgconfig.LoadFile(path)
	if err != nil {
		return pkgconfig.File{}, err
	}
	if document.Format != pkgconfig.FormatTOML {
		return pkgconfig.File{}, fmt.Errorf("Cloud config must use TOML: %s", path)
	}
	return document, nil
}

func clone(source Config) Config {
	cloned := source
	cloned.Databases = append([]DatabaseConfig(nil), source.Databases...)
	cloned.Clients.HTTP = cloneHTTPClients(source.Clients.HTTP)
	cloned.Credentials.Douyin.Headers = cloneStringMap(source.Credentials.Douyin.Headers)
	cloned.Media.FFmpeg.RequiredEncoders = append([]string(nil), source.Media.FFmpeg.RequiredEncoders...)
	return cloned
}

func cloneHTTPClients(source []httpclient.Config) []httpclient.Config {
	if source == nil {
		return nil
	}
	cloned := make([]httpclient.Config, len(source))
	copy(cloned, source)
	for index := range cloned {
		cloned[index].Endpoint.Addresses = append([]string(nil), source[index].Endpoint.Addresses...)
	}
	return cloned
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func resetForTest() {
	current.Lock()
	current.config = Config{}
	current.initialized = false
	current.Unlock()
}
