package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type ProxyCheckInput struct {
	ProxyID       string        `json:"proxy_id,omitempty"`
	ProxyProtocol ProxyProtocol `json:"proxy_protocol"`
	Host          string        `json:"host"`
	Port          int           `json:"port"`
}

type ProxyCheckResult struct {
	Connectivity string `json:"connectivity"`
}

type ProxyExtractionInput struct {
	ExtractURL    string        `json:"extract_url"`
	ProxyProtocol ProxyProtocol `json:"proxy_protocol"`
}

type ProxyExtractionResult struct {
	ProxyProtocol ProxyProtocol `json:"proxy_protocol"`
	Host          string        `json:"host"`
	Port          int           `json:"port"`
	Username      string        `json:"username,omitempty"`
	Password      string        `json:"password,omitempty"`
}

type ProxyMutationInput struct {
	Operation     string        `json:"operation,omitempty"`
	ProfileID     string        `json:"profile_id"`
	ProxyProtocol ProxyProtocol `json:"proxy_protocol"`
	Host          string        `json:"host"`
	Port          int           `json:"port"`
	Username      string        `json:"username,omitempty"`
	Password      string        `json:"password,omitempty"`
}

type ProxyMutationResult struct {
	Operation     string        `json:"operation,omitempty"`
	ProfileID     string        `json:"profile_id"`
	ProxyProtocol ProxyProtocol `json:"proxy_protocol"`
	Host          string        `json:"host"`
	Port          int           `json:"port"`
	Readback      bool          `json:"readback"`
}

type SyncProxyChecker interface {
	Check(context.Context, ProxyCheckInput) (ProxyCheckResult, error)
}

type SyncProxyExtractor interface {
	Extract(context.Context, ProxyExtractionInput) (ProxyExtractionResult, error)
}

type SyncProxyMutator interface {
	Mutate(context.Context, ProxyMutationInput) (ProxyMutationResult, error)
}

// HTTPAgentChecker calls the short-lived Agent API directly. It intentionally
// has no task creation or polling behavior.
type HTTPAgentChecker struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPAgentChecker(baseURL, token string) *HTTPAgentChecker {
	return &HTTPAgentChecker{
		baseURL: strings.TrimRight(baseURL, "/"), token: token,
		client: &http.Client{Timeout: 7 * time.Second},
	}
}

func NewHTTPAgentCheckerFromEnv() *HTTPAgentChecker {
	return NewHTTPAgentChecker(envOr("WT_MEDIA_AGENT_SYNC_URL", "http://127.0.0.1:8765"), os.Getenv("WT_MEDIA_AGENT_AUTH_TOKEN"))
}

func (c *HTTPAgentChecker) Check(ctx context.Context, input ProxyCheckInput) (ProxyCheckResult, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return ProxyCheckResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/proxy-check", strings.NewReader(string(body)))
	if err != nil {
		return ProxyCheckResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return ProxyCheckResult{}, fmt.Errorf("agent sync request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ProxyCheckResult{}, fmt.Errorf("agent sync response status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var envelope struct {
		Data ProxyCheckResult `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ProxyCheckResult{}, fmt.Errorf("decode agent sync response: %w", err)
	}
	if envelope.Data.Connectivity == "" {
		return ProxyCheckResult{}, fmt.Errorf("agent sync response missing connectivity")
	}
	return envelope.Data, nil
}

func (c *HTTPAgentChecker) Extract(ctx context.Context, input ProxyExtractionInput) (ProxyExtractionResult, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return ProxyExtractionResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/proxy-extract", strings.NewReader(string(body)))
	if err != nil {
		return ProxyExtractionResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return ProxyExtractionResult{}, fmt.Errorf("agent proxy extraction request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ProxyExtractionResult{}, fmt.Errorf("agent proxy extraction response status %d", resp.StatusCode)
	}
	var envelope struct {
		Data ProxyExtractionResult `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ProxyExtractionResult{}, fmt.Errorf("decode agent proxy extraction response: %w", err)
	}
	if envelope.Data.ProxyProtocol == "" || envelope.Data.Host == "" || envelope.Data.Port <= 0 || envelope.Data.Port > 65535 {
		return ProxyExtractionResult{}, fmt.Errorf("agent proxy extraction response missing parsed address")
	}
	return envelope.Data, nil
}

func (c *HTTPAgentChecker) Mutate(ctx context.Context, input ProxyMutationInput) (ProxyMutationResult, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return ProxyMutationResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/proxy-mutation", strings.NewReader(string(body)))
	if err != nil {
		return ProxyMutationResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return ProxyMutationResult{}, fmt.Errorf("agent proxy mutation request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ProxyMutationResult{}, fmt.Errorf("agent proxy mutation response status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var envelope struct {
		Data ProxyMutationResult `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ProxyMutationResult{}, fmt.Errorf("decode agent proxy mutation response: %w", err)
	}
	if !envelope.Data.Readback || envelope.Data.ProfileID == "" {
		return ProxyMutationResult{}, fmt.Errorf("agent proxy mutation response missing verified readback")
	}
	if input.Operation == "unbind" {
		if envelope.Data.Operation != "unbind" {
			return ProxyMutationResult{}, fmt.Errorf("agent proxy mutation response missing verified unbind")
		}
		return envelope.Data, nil
	}
	if envelope.Data.Host == "" || envelope.Data.Port <= 0 {
		return ProxyMutationResult{}, fmt.Errorf("agent proxy mutation response missing verified readback")
	}
	return envelope.Data, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
