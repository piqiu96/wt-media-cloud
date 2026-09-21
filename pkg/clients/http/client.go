// Package httpclient provides filename-derived Hertz HTTP client resources.
package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	hertzclient "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/app/client/retry"
	hertzconfig "github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// Config is the transport-only schema for one Hertz client instance.
type Config struct {
	Name       string           `toml:"name"`
	Timeout    Duration         `toml:"timeout"`
	Endpoint   EndpointConfig   `toml:"endpoint"`
	Connection ConnectionConfig `toml:"connection"`
	Retry      RetryConfig      `toml:"retry"`
}

// Duration parses a quoted Go duration from TOML.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(value []byte) error {
	parsed, err := time.ParseDuration(string(value))
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

// ConnectionConfig maps explicit Hertz connection options.
type ConnectionConfig struct {
	DialTimeout         Duration `toml:"dial_timeout"`
	ReadTimeout         Duration `toml:"read_timeout"`
	WriteTimeout        Duration `toml:"write_timeout"`
	MaxConnsPerHost     int      `toml:"max_conns_per_host"`
	MaxIdleConnDuration Duration `toml:"max_idle_conn_duration"`
	MaxConnDuration     Duration `toml:"max_conn_duration"`
	MaxConnWaitTimeout  Duration `toml:"max_conn_wait_timeout"`
	KeepAlive           bool     `toml:"keep_alive"`
}

// RetryConfig maps Hertz retry behavior.
type RetryConfig struct {
	Attempts int      `toml:"attempts"`
	Delay    Duration `toml:"delay"`
	MaxDelay Duration `toml:"max_delay"`
	Policy   string   `toml:"policy"`
}

// Client wraps Hertz Client with a whole-request timeout.
type Client struct {
	hertz    *hertzclient.Client
	timeout  time.Duration
	endpoint EndpointConfig
}

// New builds one validated Hertz client and applies middleware in order.
func New(cfg Config, middlewares ...hertzclient.Middleware) (*Client, func() error, error) {
	if err := Validate(cfg); err != nil {
		return nil, nil, err
	}

	options := []hertzconfig.ClientOption{
		hertzclient.WithDialTimeout(cfg.Connection.DialTimeout.Duration),
		hertzclient.WithClientReadTimeout(cfg.Connection.ReadTimeout.Duration),
		hertzclient.WithWriteTimeout(cfg.Connection.WriteTimeout.Duration),
		hertzclient.WithMaxConnsPerHost(cfg.Connection.MaxConnsPerHost),
		hertzclient.WithMaxIdleConnDuration(cfg.Connection.MaxIdleConnDuration.Duration),
		hertzclient.WithMaxConnDuration(cfg.Connection.MaxConnDuration.Duration),
		hertzclient.WithMaxConnWaitTimeout(cfg.Connection.MaxConnWaitTimeout.Duration),
		hertzclient.WithKeepAlive(cfg.Connection.KeepAlive),
		hertzclient.WithRetryConfig(retryOptions(cfg.Retry)...),
	}
	if cfg.Endpoint.Scheme == "https" {
		options = append(options, hertzclient.WithTLSConfig(&tls.Config{
			ServerName: cfg.Endpoint.Host,
			MinVersion: tls.VersionTLS12,
		}))
	}
	if endpointDialer := newEndpointDialer(cfg.Endpoint); endpointDialer != nil {
		options = append(options, hertzclient.WithDialer(endpointDialer))
	}

	raw, err := hertzclient.NewClient(options...)
	if err != nil {
		return nil, nil, fmt.Errorf("build hertz client: %w", err)
	}
	raw.RetryIfFunc = func(_ *protocol.Request, _ *protocol.Response, err error) bool {
		return err != nil
	}
	for _, middleware := range middlewares {
		raw.Use(middleware)
	}

	return &Client{hertz: raw, timeout: cfg.Timeout.Duration, endpoint: cfg.Endpoint}, func() error {
		raw.CloseIdleConnections()
		return nil
	}, nil
}

// Do performs a request with the configured whole-request timeout.
func (c *Client) Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	if c.timeout <= 0 {
		return c.hertz.Do(ctx, req, resp)
	}
	return c.hertz.DoTimeout(ctx, req, resp, c.timeout)
}

// Origin returns the configured logical URL origin.
func (c *Client) Origin() string {
	return c.endpoint.Origin()
}

// CloseIdleConnections releases pooled connections owned by this client.
func (c *Client) CloseIdleConnections() {
	c.hertz.CloseIdleConnections()
}

func retryOptions(cfg RetryConfig) []retry.Option {
	policy := retry.FixedDelayPolicy
	if cfg.Policy == "backoff" {
		policy = retry.BackOffDelayPolicy
	}
	return []retry.Option{
		retry.WithMaxAttemptTimes(uint(cfg.Attempts)),
		retry.WithInitDelay(cfg.Delay.Duration),
		retry.WithMaxDelay(cfg.MaxDelay.Duration),
		retry.WithDelayPolicy(policy),
	}
}

// Validate reports whether one HTTP client configuration is complete and safe.
func Validate(cfg Config) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("http client name is required")
	}
	if cfg.Timeout.Duration <= 0 {
		return errors.New("http client timeout must be greater than zero")
	}
	if cfg.Retry.Attempts <= 0 {
		return errors.New("http client retry.attempts must be greater than zero")
	}
	if cfg.Retry.Delay.Duration < 0 || cfg.Retry.MaxDelay.Duration < 0 {
		return errors.New("http client retry delays must not be negative")
	}
	if retryPolicyInvalid(cfg.Retry.Policy) {
		return fmt.Errorf("invalid http client retry policy %q", cfg.Retry.Policy)
	}
	if err := cfg.Endpoint.validate(); err != nil {
		return err
	}
	connection := cfg.Connection
	if connection.DialTimeout.Duration < 0 || connection.ReadTimeout.Duration < 0 || connection.WriteTimeout.Duration < 0 ||
		connection.MaxIdleConnDuration.Duration < 0 || connection.MaxConnDuration.Duration < 0 || connection.MaxConnWaitTimeout.Duration < 0 ||
		connection.MaxConnsPerHost < 0 {
		return errors.New("http client connection values must not be negative")
	}
	return nil
}

func retryPolicyInvalid(policy string) bool {
	return policy != "" && policy != "fixed" && policy != "backoff"
}
