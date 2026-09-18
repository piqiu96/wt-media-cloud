// Package config loads, validates, and exposes read-only process configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
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
	Name         string             `yaml:"name"`
	Server       ServerConfig       `yaml:"server"`
	InitialAdmin InitialAdminConfig `yaml:"initial_admin"`
}

type ServerConfig struct {
	HTTPAddr            string `yaml:"http_addr"`
	SessionCookieSecure bool   `yaml:"session_cookie_secure"`
}

type InitialAdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type DatabaseConfig struct {
	Name      string             `yaml:"name"`
	Host      string             `yaml:"host"`
	Port      int                `yaml:"port"`
	Database  string             `yaml:"database"`
	Username  string             `yaml:"username"`
	Password  string             `yaml:"password"`
	Charset   string             `yaml:"charset"`
	ParseTime bool               `yaml:"parse_time"`
	Location  string             `yaml:"location"`
	Pool      DatabasePoolConfig `yaml:"pool"`
}

type DatabasePoolConfig struct {
	MaxIdle     int      `yaml:"max_idle"`
	MaxOpen     int      `yaml:"max_open"`
	MaxLifetime Duration `yaml:"max_lifetime"`
}

type CacheConfig struct {
	Redis RedisConfig
}

type RedisConfig struct {
	URL string `yaml:"url"`
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
	Path   string `yaml:"path"`
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type ClientsConfig struct {
	Agent  ClientConfig
	Douyin ClientConfig
}

type ClientConfig struct {
	Name    string      `yaml:"name"`
	Scheme  string      `yaml:"scheme"`
	Host    string      `yaml:"host"`
	Port    int         `yaml:"port"`
	Timeout Duration    `yaml:"timeout"`
	Retry   RetryConfig `yaml:"retry"`
}

type RetryConfig struct {
	Attempts int      `yaml:"attempts"`
	Interval Duration `yaml:"interval"`
}

type CredentialsConfig struct {
	Agent  AgentCredentialConfig
	Douyin DouyinCredentialConfig
}

type AgentCredentialConfig struct {
	AuthToken string `yaml:"auth_token"`
}

type DouyinCredentialConfig struct {
	APIKey  string            `yaml:"api_key"`
	Cookie  string            `yaml:"cookie"`
	Headers map[string]string `yaml:"headers"`
}

type SchedulerConfig struct {
	ProxyExpiryInterval Duration `yaml:"proxy_expiry_interval"`
	DiscoveryInterval   Duration `yaml:"discovery_interval"`
	WorkerInterval      Duration `yaml:"worker_interval"`
	WorkerBatchSize     int      `yaml:"worker_batch_size"`
}

type ObservabilityConfig struct {
	HealthPath string `yaml:"health_path"`
}

// Duration is a strict YAML duration parsed with time.ParseDuration.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a scalar")
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(node.Value))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
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
	if err := decodeRequired(filepath.Join(root, "app.yaml"), &cfg.App); err != nil {
		return Config{}, err
	}

	databases, err := loadDatabases(filepath.Join(root, "database"))
	if err != nil {
		return Config{}, err
	}
	cfg.Databases = databases

	if err := decodeOptional(filepath.Join(root, "cache", "redis.yaml"), &cfg.Cache.Redis); err != nil {
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
		if err := decodeRequired(filepath.Join(root, "logger", file.name+".yaml"), file.dst); err != nil {
			return Config{}, err
		}
	}
	if err := decodeRequired(filepath.Join(root, "clients", "agent.yaml"), &cfg.Clients.Agent); err != nil {
		return Config{}, err
	}
	if err := decodeRequired(filepath.Join(root, "clients", "platforms", "douyin.yaml"), &cfg.Clients.Douyin); err != nil {
		return Config{}, err
	}
	if err := decodeOptional(filepath.Join(root, "credentials", "agent.yaml"), &cfg.Credentials.Agent); err != nil {
		return Config{}, err
	}
	if err := decodeOptional(filepath.Join(root, "credentials", "douyin.yaml"), &cfg.Credentials.Douyin); err != nil {
		return Config{}, err
	}
	if err := decodeRequired(filepath.Join(root, "scheduler", "scheduler.yaml"), &cfg.Scheduler); err != nil {
		return Config{}, err
	}
	if err := decodeOptional(filepath.Join(root, "observability", "health.yaml"), &cfg.Observability); err != nil {
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
		if logger.Format != "json" && logger.Format != "text" {
			return fmt.Errorf("logger.%s.format must be json or text", name)
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
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("database %q is required: directory %s does not exist", defaultDatabaseName, directory)
	}
	if err != nil {
		return nil, fmt.Errorf("read config directory %s: %w", directory, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	databases := make([]DatabaseConfig, 0, len(entries))
	seen := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml") || entry.Name() == "migration.yaml" || entry.Name() == "migration.yml" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		var database DatabaseConfig
		if err := decodeRequired(path, &database); err != nil {
			return nil, err
		}
		if previous, duplicate := seen[database.Name]; duplicate {
			return nil, fmt.Errorf("duplicate database name %q in %s and %s", database.Name, previous, path)
		}
		seen[database.Name] = path
		databases = append(databases, database)
	}
	return databases, nil
}

func decodeRequired(path string, dst any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	return decode(path, contents, dst)
}

func decodeOptional(path string, dst any) error {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	return decode(path, contents, dst)
}

func decode(path string, contents []byte, dst any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode config %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode config %s: multiple YAML documents are not allowed", path)
		}
		return fmt.Errorf("decode config %s: %w", path, err)
	}
	return nil
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
