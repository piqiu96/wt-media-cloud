// Package deepseek owns the single configured model HTTP boundary.
package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ErrNotConfigured = errors.New("DeepSeek not configured")

type Client struct {
	transport  *http.Client
	key, model string
	origin     string
}

var resource struct {
	sync.RWMutex
	client *Client
}

func Initialize(settings config.PublicationGenerationConfig, credentials config.DeepSeekCredentialConfig) (func() error, error) {
	resource.Lock()
	defer resource.Unlock()
	if resource.client != nil {
		return nil, errors.New("DeepSeek already initialized")
	}
	transport := &http.Transport{MaxConnsPerHost: 4, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: settings.Timeout.Duration}
	resource.client = NewWithClient(&http.Client{Transport: transport, Timeout: settings.Timeout.Duration, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, settings.BaseURL, credentials.APIKey, settings.Model)
	return Close, nil
}
func Close() error {
	resource.Lock()
	if resource.client != nil {
		resource.client.transport.CloseIdleConnections()
	}
	resource.client = nil
	resource.Unlock()
	return nil
}
func Get() *Client { resource.RLock(); defer resource.RUnlock(); return resource.client }
func NewWithClient(transport *http.Client, origin, key, model string) *Client {
	return &Client{transport: transport, origin: strings.TrimRight(origin, "/"), key: key, model: model}
}

func (c *Client) Ready() bool { return c != nil && c.transport != nil && c.key != "" && c.model != "" }

// CompleteJSON never returns partial output or retries a paid generation POST.
// Provider bodies, prompts and credentials are deliberately absent from errors.
func (c *Client) CompleteJSON(ctx context.Context, system, user string) (string, error) {
	if !c.Ready() {
		return "", ErrNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]any{"model": c.model, "stream": false, "thinking": map[string]string{"type": "disabled"}, "response_format": map[string]string{"type": "json_object"}, "max_tokens": 2048, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}})
	if err != nil {
		return "", errors.New("encode model request failed")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.origin+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", errors.New("model request invalid")
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.transport.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("model request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("model response status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024+1))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("model response read failed")
	}
	if len(data) > 128*1024 {
		return "", errors.New("model response exceeds limit")
	}

	var result struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role    string          `json:"role"`
				Content string          `json:"content"`
				Refusal json.RawMessage `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &result) != nil || len(result.Choices) != 1 {
		return "", errors.New("model response invalid")
	}
	choice := result.Choices[0]
	content := strings.TrimSpace(choice.Message.Content)
	refusal := string(choice.Message.Refusal)
	if choice.FinishReason != "stop" || choice.Message.Role != "assistant" || content == "" || !json.Valid([]byte(content)) || refusal != "" && refusal != "null" && refusal != `""` {
		return "", errors.New("model response incomplete")
	}
	return content, nil
}
