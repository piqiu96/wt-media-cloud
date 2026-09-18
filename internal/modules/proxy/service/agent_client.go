package service

import (
	"context"
	"fmt"
	agentclient "github.com/wt-media/wt-media-cloud/internal/infra/client/agent"
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
	client *agentclient.Client
}

func NewAgentChecker() *HTTPAgentChecker {
	return &HTTPAgentChecker{client: agentclient.Get()}
}

func NewAgentCheckerWithClient(client *agentclient.Client) *HTTPAgentChecker {
	return &HTTPAgentChecker{client: client}
}

func (c *HTTPAgentChecker) Check(ctx context.Context, input ProxyCheckInput) (ProxyCheckResult, error) {
	result, err := c.client.CheckProxy(ctx, agentclient.ProxyCheckRequest{
		ProxyID:       input.ProxyID,
		ProxyProtocol: string(input.ProxyProtocol),
		Host:          input.Host,
		Port:          input.Port,
	})
	if err != nil {
		return ProxyCheckResult{}, err
	}
	if result.Connectivity == "" {
		return ProxyCheckResult{}, fmt.Errorf("agent sync response missing connectivity")
	}
	return ProxyCheckResult{Connectivity: result.Connectivity}, nil
}

func (c *HTTPAgentChecker) Extract(ctx context.Context, input ProxyExtractionInput) (ProxyExtractionResult, error) {
	result, err := c.client.ExtractProxy(ctx, agentclient.ProxyExtractionRequest{
		ExtractURL:    input.ExtractURL,
		ProxyProtocol: string(input.ProxyProtocol),
	})
	if err != nil {
		return ProxyExtractionResult{}, err
	}
	if result.ProxyProtocol == "" || result.Host == "" || result.Port <= 0 || result.Port > 65535 {
		return ProxyExtractionResult{}, fmt.Errorf("agent proxy extraction response missing parsed address")
	}
	return ProxyExtractionResult{
		ProxyProtocol: ProxyProtocol(result.ProxyProtocol),
		Host:          result.Host,
		Port:          result.Port,
		Username:      result.Username,
		Password:      result.Password,
	}, nil
}

func (c *HTTPAgentChecker) Mutate(ctx context.Context, input ProxyMutationInput) (ProxyMutationResult, error) {
	result, err := c.client.MutateProxy(ctx, agentclient.ProxyMutationRequest{
		Operation:     input.Operation,
		ProfileID:     input.ProfileID,
		ProxyProtocol: string(input.ProxyProtocol),
		Host:          input.Host,
		Port:          input.Port,
		Username:      input.Username,
		Password:      input.Password,
	})
	if err != nil {
		return ProxyMutationResult{}, err
	}
	if !result.Readback || result.ProfileID == "" {
		return ProxyMutationResult{}, fmt.Errorf("agent proxy mutation response missing verified readback")
	}
	if input.Operation == "unbind" {
		if result.Operation != "unbind" {
			return ProxyMutationResult{}, fmt.Errorf("agent proxy mutation response missing verified unbind")
		}
		return proxyMutationResultFromAgent(result), nil
	}
	if result.Host == "" || result.Port <= 0 {
		return ProxyMutationResult{}, fmt.Errorf("agent proxy mutation response missing verified readback")
	}
	return proxyMutationResultFromAgent(result), nil
}

func proxyMutationResultFromAgent(result agentclient.ProxyMutationResult) ProxyMutationResult {
	return ProxyMutationResult{
		Operation:     result.Operation,
		ProfileID:     result.ProfileID,
		ProxyProtocol: ProxyProtocol(result.ProxyProtocol),
		Host:          result.Host,
		Port:          result.Port,
		Readback:      result.Readback,
	}
}
