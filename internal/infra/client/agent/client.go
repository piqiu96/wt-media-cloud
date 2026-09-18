// Package agent implements the Cloud-to-Agent HTTP protocol.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

type ProxyCheckRequest struct {
	ProxyID       string `json:"proxy_id,omitempty"`
	ProxyProtocol string `json:"proxy_protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
}

type ProxyCheckResult struct {
	Connectivity string `json:"connectivity"`
}

type ProxyExtractionRequest struct {
	ExtractURL    string `json:"extract_url"`
	ProxyProtocol string `json:"proxy_protocol"`
}

type ProxyExtractionResult struct {
	ProxyProtocol string `json:"proxy_protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
}

type ProxyMutationRequest struct {
	Operation     string `json:"operation,omitempty"`
	ProfileID     string `json:"profile_id"`
	ProxyProtocol string `json:"proxy_protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
}

type ProxyMutationResult struct {
	Operation     string `json:"operation,omitempty"`
	ProfileID     string `json:"profile_id"`
	ProxyProtocol string `json:"proxy_protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Readback      bool   `json:"readback"`
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	attempts   int
	interval   time.Duration
}

type resourceState struct {
	sync.RWMutex
	client *Client
}

var resources resourceState

// Initialize validates transport configuration, builds the Agent client, and publishes it.
func Initialize(connection config.ClientConfig, credential config.AgentCredentialConfig) error {
	if err := validateConnection(connection); err != nil {
		return err
	}
	client := NewWithHTTPClient(connection, credential, nil)
	resources.Lock()
	defer resources.Unlock()
	if resources.client != nil {
		return errors.New("agent client already initialized")
	}
	resources.client = client
	return nil
}

// Get returns the initialized Agent client.
func Get() *Client {
	resources.RLock()
	client := resources.client
	resources.RUnlock()
	if client == nil {
		panic("agent: Get called before Initialize")
	}
	return client
}

// Close releases the initialized Agent client and is safe to call repeatedly.
func Close() error {
	resources.Lock()
	client := resources.client
	resources.client = nil
	resources.Unlock()
	if client == nil {
		return nil
	}
	client.httpClient.CloseIdleConnections()
	return nil
}

// NewWithHTTPClient builds a testable Agent client with an injected HTTP transport.
func NewWithHTTPClient(connection config.ClientConfig, credential config.AgentCredentialConfig, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	httpClient.Timeout = connection.Timeout.Duration
	attempts := connection.Retry.Attempts
	if attempts < 1 {
		attempts = 1
	}
	return &Client{
		baseURL:    buildBaseURL(connection),
		token:      credential.AuthToken,
		httpClient: httpClient,
		attempts:   attempts,
		interval:   connection.Retry.Interval.Duration,
	}
}

// CheckProxy asks the local Agent to test a proxy address.
func (c *Client) CheckProxy(ctx context.Context, request ProxyCheckRequest) (ProxyCheckResult, error) {
	var result ProxyCheckResult
	err := c.post(ctx, "/api/v1/proxy-check", request, &result)
	return result, err
}

// ExtractProxy asks the local Agent to extract a proxy address.
func (c *Client) ExtractProxy(ctx context.Context, request ProxyExtractionRequest) (ProxyExtractionResult, error) {
	var result ProxyExtractionResult
	err := c.post(ctx, "/api/v1/proxy-extract", request, &result)
	return result, err
}

// MutateProxy asks the local Agent to bind or unbind a profile proxy.
func (c *Client) MutateProxy(ctx context.Context, request ProxyMutationRequest) (ProxyMutationResult, error) {
	var result ProxyMutationResult
	err := c.post(ctx, "/api/v1/proxy-mutation", request, &result)
	return result, err
}

func (c *Client) post(ctx context.Context, path string, input, output any) error {
	return doRequest(ctx, c.httpClient, c.attempts, c.interval, func() (*http.Request, error) {
		body, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		if c.token != "" {
			request.Header.Set("Authorization", "Bearer "+c.token)
		}
		return request, nil
	}, func(response *http.Response) error {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("decode agent response: %w", err)
		}
		if err := json.Unmarshal(envelope.Data, output); err != nil {
			return fmt.Errorf("decode agent response data: %w", err)
		}
		return nil
	})
}

func doRequest(
	ctx context.Context,
	httpClient *http.Client,
	attempts int,
	interval time.Duration,
	newRequest func() (*http.Request, error),
	handleSuccess func(*http.Response) error,
) error {
	for attempt := 1; attempt <= attempts; attempt++ {
		request, err := newRequest()
		if err != nil {
			return err
		}
		response, err := httpClient.Do(request)
		if err != nil {
			if attempt == attempts {
				return err
			}
			if err := sleepWithContext(ctx, interval); err != nil {
				return err
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
			_ = response.Body.Close()
			return fmt.Errorf("agent response status %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
		}
		err = handleSuccess(response)
		_ = response.Body.Close()
		return err
	}
	return nil
}

func sleepWithContext(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateConnection(connection config.ClientConfig) error {
	if connection.Scheme != "http" && connection.Scheme != "https" {
		return fmt.Errorf("agent client scheme must be http or https")
	}
	if strings.TrimSpace(connection.Host) == "" {
		return errors.New("agent client host is required")
	}
	if connection.Port <= 0 || connection.Port > 65535 {
		return errors.New("agent client port is invalid")
	}
	if connection.Timeout.Duration <= 0 {
		return errors.New("agent client timeout must be greater than zero")
	}
	if connection.Retry.Attempts < 1 {
		return errors.New("agent client retry attempts must be greater than zero")
	}
	if connection.Retry.Interval.Duration < 0 {
		return errors.New("agent client retry interval cannot be negative")
	}
	return nil
}

func buildBaseURL(connection config.ClientConfig) string {
	host := connection.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return connection.Scheme + "://" + host + ":" + strconv.Itoa(connection.Port)
}
