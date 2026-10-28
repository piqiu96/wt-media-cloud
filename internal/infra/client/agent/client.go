// Package agent implements the Cloud-to-Agent HTTP protocol.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/config"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
)

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

type ResponseError struct {
	Status int
	Body   string
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("agent response status %d: %s", e.Status, e.Body)
}

type Client struct {
	origin    string
	token     string
	transport *httpclient.Client
}

type resourceState struct {
	sync.RWMutex
	client *Client
}

var resources resourceState

// Initialize builds the Agent client from its named Hertz transport and publishes it.
func Initialize(name string, credentials config.CredentialsConfig) (func() error, error) {
	transport := httpclient.Get(name)
	client := NewWithClient(transport.Origin(), credentials.Agent, transport)
	resources.Lock()
	defer resources.Unlock()
	if resources.client != nil {
		return nil, errors.New("agent client already initialized")
	}
	resources.client = client
	return Close, nil
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
func NewWithClient(origin string, credential config.AgentCredentialConfig, transport *httpclient.Client) *Client {
	if transport == nil {
		panic("agent: Hertz transport is required")
	}
	return &Client{
		origin:    strings.TrimRight(origin, "/"),
		token:     credential.AuthToken,
		transport: transport,
	}
}

// ExtractProxy asks the local Agent to extract a proxy address.
func (c *Client) ExtractProxy(ctx context.Context, request ProxyExtractionRequest) (ProxyExtractionResult, error) {
	var result ProxyExtractionResult
	err := c.post(ctx, "/api/v1/proxy-extract", request, &result)
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
	request.SetRequestURI(c.origin + path)
	request.SetBodyRaw(body)
	request.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	if err := c.transport.Do(ctx, request, response); err != nil {
		return err
	}
	if response.StatusCode() < 200 || response.StatusCode() >= 300 {
		return &ResponseError{Status: response.StatusCode(), Body: strings.TrimSpace(string(response.BodyBytes()))}
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
