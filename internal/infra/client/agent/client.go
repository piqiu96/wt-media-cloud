// Package agent implements the Cloud-to-Agent HTTP protocol.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/config"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
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
	baseURL   string
	token     string
	transport *httpclient.Client
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
	client := NewWithClient(strings.TrimRight(connection.BaseURL, "/"), credential, httpclient.Get(connection.Name))
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
	return nil
}

// NewWithClient builds a testable Agent client over an initialized Hertz transport.
func NewWithClient(baseURL string, credential config.AgentCredentialConfig, transport *httpclient.Client) *Client {
	if transport == nil {
		panic("agent: Hertz transport is required")
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     credential.AuthToken,
		transport: transport,
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
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}

	request := protocol.AcquireRequest()
	response := protocol.AcquireResponse()
	defer protocol.ReleaseRequest(request)
	defer protocol.ReleaseResponse(response)
	request.SetMethod("POST")
	request.SetRequestURI(c.baseURL + path)
	request.SetBodyRaw(body)
	request.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	if err := c.transport.Do(ctx, request, response); err != nil {
		return err
	}
	if response.StatusCode() < 200 || response.StatusCode() >= 300 {
		return fmt.Errorf("agent response status %d: %s", response.StatusCode(), strings.TrimSpace(string(response.BodyBytes())))
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.BodyBytes(), &envelope); err != nil {
		return fmt.Errorf("decode agent response: %w", err)
	}
	if err := json.Unmarshal(envelope.Data, output); err != nil {
		return fmt.Errorf("decode agent response data: %w", err)
	}
	return nil
}

func validateConnection(connection config.ClientConfig) error {
	if strings.TrimSpace(connection.Name) == "" {
		return errors.New("agent client name is required")
	}
	if strings.TrimSpace(connection.BaseURL) == "" {
		return errors.New("agent client base_url is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(connection.BaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("agent client base_url must be an absolute http or https URL")
	}
	if connection.Timeout.Duration <= 0 {
		return errors.New("agent client timeout must be greater than zero")
	}
	return nil
}
