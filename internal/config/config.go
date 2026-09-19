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
	App           AppConfig
	Databases     []DatabaseConfig
	Cache         CacheConfig
	Loggers       LoggerConfigs
	Clients       ClientsConfig
	Credentials   CredentialsConfig
	Scheduler     SchedulerConfig
	Observability ObservabilityConfig
}

type AppConfig struct {
	Name         string             `toml:"name"`
	Server       ServerConfig       `toml:"server"`
	InitialAdmin InitialAdminConfig `toml:"initial_admin"`
}

type ServerConfig struct {
	HTTPAddr            string `toml:"http_addr"`
	SessionCookieSecure bool   `toml:"session_cookie_secure"`
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

type CacheConfig struct {
	Redis RedisConfig
}

type RedisConfig struct {
	URL string `toml:"url"`
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
	Agent  ClientConfig
	Douyin ClientConfig
}

type ClientConfig struct {
	Name    string      `toml:"name"`
	Scheme  string      `toml:"scheme"`
	Host    string      `toml:"host"`
	Port    int         `toml:"port"`
	Timeout Duration    `toml:"timeout"`
	Retry   RetryConfig `toml:"retry"`
}

type RetryConfig struct {
	Attempts int      `toml:"attempts"`
	Interval Duration `toml:"interval"`
}

type CredentialsConfig struct {
	Agent  AgentCredentialConfig
	Douyin DouyinCredentialConfig
}

type AgentCredentialConfig struct {
	AuthToken string `toml:"auth_token"`
}

type DouyinCredentialConfig struct {
	APIKey  string            `toml:"api_key"`
	Cookie  string            `toml:"cookie"`
	Headers map[string]string `toml:"headers"`
}

type SchedulerConfig struct {
	ProxyExpiryInterval Duration `toml:"proxy_expiry_interval"`
	DiscoveryInterval   Duration `toml:"discovery_interval"`
	WorkerInterval      Duration `toml:"worker_interval"`
	WorkerBatchSize     int      `toml:"worker_batch_size"`
}

type ObservabilityConfig struct {
	HealthPath string `toml:"health_path"`
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

// Initialize loads ./config, validates it, and atomically publishes a copy.
func Initialize() error {
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

// Load always reads the runtime configuration directory ./config.
func Load() (Config, error) {
	return LoadFromDir(configDirectory)
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

	if err := optionalTOML(filepath.Join(root, "cache", "redis.toml"), &cfg.Cache.Redis); err != nil {
		return Config{}, err
	}
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
	if err := requiredTOML(filepath.Join(root, "clients", "agent.toml"), &cfg.Clients.Agent); err != nil {
		return Config{}, err
	}
	if err := requiredTOML(filepath.Join(root, "clients", "platforms", "douyin.toml"), &cfg.Clients.Douyin); err != nil {
		return Config{}, err
	}
	if err := optionalTOML(filepath.Join(root, "credentials", "agent.toml"), &cfg.Credentials.Agent); err != nil {
		return Config{}, err
	}
	if err := optionalTOML(filepath.Join(root, "credentials", "douyin.toml"), &cfg.Credentials.Douyin); err != nil {
		return Config{}, err
	}
	if err := requiredTOML(filepath.Join(root, "scheduler", "scheduler.toml"), &cfg.Scheduler); err != nil {
		return Config{}, err
	}
	if err := optionalTOML(filepath.Join(root, "observability", "health.toml"), &cfg.Observability); err != nil {
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
	if err := validateClient("agent", c.Clients.Agent); err != nil {
		return err
	}
	if err := validateClient("douyin", c.Clients.Douyin); err != nil {
		return err
	}
	if c.Scheduler.ProxyExpiryInterval.Duration <= 0 {
		return errors.New("scheduler.proxy_expiry_interval must be greater than zero")
	}
	if c.Scheduler.DiscoveryInterval.Duration <= 0 {
		return errors.New("scheduler.discovery_interval must be greater than zero")
	}
	if c.Scheduler.WorkerInterval.Duration <= 0 {
		return errors.New("scheduler.worker_interval must be greater than zero")
	}
	if c.Scheduler.WorkerBatchSize <= 0 {
		return errors.New("scheduler.worker_batch_size must be greater than zero")
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

func validateClient(name string, client ClientConfig) error {
	if client.Name == "" || client.Scheme == "" || client.Host == "" || client.Port <= 0 {
		return fmt.Errorf("client.%s requires name, scheme, host, and port", name)
	}
	if client.Scheme != "http" && client.Scheme != "https" {
		return fmt.Errorf("client.%s.scheme must be http or https", name)
	}
	if client.Timeout.Duration <= 0 {
		return fmt.Errorf("client.%s.timeout must be greater than zero", name)
	}
	if client.Retry.Attempts <= 0 {
		return fmt.Errorf("client.%s.retry.attempts must be greater than zero", name)
	}
	if client.Retry.Interval.Duration < 0 {
		return fmt.Errorf("client.%s.retry.interval must not be negative", name)
	}
	return nil
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
	document, err := loadCloudDocument(path)
	if err != nil {
		return err
	}
	if err := decodeDocument(path, document, dst); err != nil {
		return err
	}
	return nil
}

func optionalTOML(path string, dst any) error {
	document, err := loadCloudDocument(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := decodeDocument(path, document, dst); err != nil {
		return err
	}
	return nil
}

func decodeDocument(path string, document pkgconfig.Document, dst any) error {
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

func loadCloudDocument(path string) (pkgconfig.Document, error) {
	document, err := pkgconfig.LoadFile(path)
	if err != nil {
		return pkgconfig.Document{}, err
	}
	if document.Format != pkgconfig.FormatTOML {
		return pkgconfig.Document{}, fmt.Errorf("Cloud config must use TOML: %s", path)
	}
	return document, nil
}

func clone(source Config) Config {
	cloned := source
	cloned.Databases = append([]DatabaseConfig(nil), source.Databases...)
	cloned.Credentials.Douyin.Headers = cloneStringMap(source.Credentials.Douyin.Headers)
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
