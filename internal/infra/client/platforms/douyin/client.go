// Package douyin owns the external Douyin HTTP protocol.
package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/config"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
)

type SearchRequest struct {
	Keyword string
	Count   int
	Offset  int
}

type FindAuthorRequest struct {
	SecUID    string
	Count     int
	MaxCursor int64
}

type FetchByURLRequest struct {
	URL string
}

type FetchByIDsRequest struct {
	IDs []string
}

type Client struct {
	baseURL   string
	apiKey    string
	cookie    string
	headers   map[string]string
	transport *httpclient.Client
}

type resourceState struct {
	sync.RWMutex
	client *Client
}

var resources resourceState

// Initialize validates transport configuration, builds the Douyin client, and publishes it.
func Initialize(connection config.ClientConfig, credential config.DouyinCredentialConfig) error {
	if err := validateConnection(connection); err != nil {
		return err
	}
	client := NewWithClient(strings.TrimRight(connection.BaseURL, "/"), credential, httpclient.Get(connection.Name))
	resources.Lock()
	defer resources.Unlock()
	if resources.client != nil {
		return errors.New("douyin client already initialized")
	}
	resources.client = client
	return nil
}

// Get returns the initialized Douyin client.
func Get() *Client {
	resources.RLock()
	client := resources.client
	resources.RUnlock()
	if client == nil {
		panic("douyin: Get called before Initialize")
	}
	return client
}

// Close releases the initialized Douyin client and is safe to call repeatedly.
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

// NewWithClient builds a testable Douyin client over an initialized Hertz transport.
func NewWithClient(baseURL string, credential config.DouyinCredentialConfig, transport *httpclient.Client) *Client {
	if transport == nil {
		panic("douyin: Hertz transport is required")
	}
	headers := make(map[string]string, len(credential.Headers))
	for key, value := range credential.Headers {
		if strings.TrimSpace(key) != "" {
			headers[key] = value
		}
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    credential.APIKey,
		cookie:    credential.Cookie,
		headers:   headers,
		transport: transport,
	}
}

// Configured reports whether the separately loaded Douyin API key is present.
func (c *Client) Configured() bool {
	return c != nil && strings.TrimSpace(c.apiKey) != ""
}

// Search requests keyword video results from the public Douyin data API.
func (c *Client) Search(ctx context.Context, request SearchRequest) (map[string]any, error) {
	values := map[string]string{
		"keywords": request.Keyword,
		"count":    strconv.Itoa(clamp(request.Count, 1, 30)),
		"offset":   strconv.Itoa(max(request.Offset, 0)),
	}
	if c.cookie != "" {
		values["ck"] = c.cookie
	}
	var payload map[string]any
	err := c.postForm(ctx, "/v2dysearchvideo", values, &payload)
	return payload, err
}

// FindAuthor requests an author's videos by sec_uid from the public Douyin data API.
func (c *Client) FindAuthor(ctx context.Context, request FindAuthorRequest) (map[string]any, error) {
	values := map[string]string{
		"sec_uid":    strings.TrimSpace(request.SecUID),
		"count":      strconv.Itoa(clamp(request.Count, 1, 20)),
		"max_cursor": strconv.FormatInt(request.MaxCursor, 10),
	}
	if c.cookie != "" {
		values["ck"] = c.cookie
	}
	var payload map[string]any
	err := c.postForm(ctx, "/v5/dyhome", values, &payload)
	return payload, err
}

// FetchByIDs requests details for up to ten Douyin video IDs in one call.
func (c *Client) FetchByIDs(ctx context.Context, request FetchByIDsRequest) (map[string]any, error) {
	ids := make([]string, 0, len(request.IDs))
	seen := make(map[string]struct{}, len(request.IDs))
	for _, id := range request.IDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return map[string]any{}, errors.New("douyin video IDs are required")
	}
	if len(ids) > 10 {
		return map[string]any{}, errors.New("douyin video IDs exceed batch limit of 10")
	}
	var payload map[string]any
	err := c.postForm(ctx, "/batchDyVideo", map[string]string{"ids": strings.Join(ids, ",")}, &payload)
	return payload, err
}

// FetchByURL requests a Douyin video by numeric ID or short URL.
func (c *Client) FetchByURL(ctx context.Context, request FetchByURLRequest) (map[string]any, error) {
	source := strings.TrimSpace(request.URL)
	values := map[string]string{}
	if _, err := strconv.ParseUint(source, 10, 64); err == nil {
		values["id"] = source
	} else {
		values["shorturl"] = source
	}
	var payload map[string]any
	err := c.postForm(ctx, "/dyVideo/detail", values, &payload)
	return payload, err
}

func (c *Client) postForm(ctx context.Context, path string, fields map[string]string, target *map[string]any) error {
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	endpoint := c.baseURL + path + "?apiKey=" + url.QueryEscape(c.apiKey)

	request := protocol.AcquireRequest()
	response := protocol.AcquireResponse()
	defer protocol.ReleaseRequest(request)
	defer protocol.ReleaseResponse(response)
	request.SetMethod("POST")
	request.SetRequestURI(endpoint)
	request.SetBodyRaw([]byte(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "WT-Media-Cloud/1")
	for key, value := range c.headers {
		request.Header.Set(key, value)
	}
	if c.cookie != "" {
		request.Header.Set("Cookie", c.cookie)
	}

	if err := c.transport.Do(ctx, request, response); err != nil {
		return err
	}
	if response.StatusCode() < 200 || response.StatusCode() >= 300 {
		return fmt.Errorf("douyin http status %d: %s", response.StatusCode(), strings.TrimSpace(string(response.BodyBytes())))
	}
	if err := json.Unmarshal(response.BodyBytes(), target); err != nil {
		return fmt.Errorf("decode douyin response: %w", err)
	}
	if !successValue((*target)["result"]) {
		return errors.New("douyin API rejected request")
	}
	return nil
}

func successValue(value any) bool {
	switch typed := value.(type) {
	case float64:
		return typed == 1
	case int:
		return typed == 1
	case string:
		return typed == "1"
	default:
		return false
	}
}

func validateConnection(connection config.ClientConfig) error {
	if strings.TrimSpace(connection.Name) == "" {
		return errors.New("douyin client name is required")
	}
	if strings.TrimSpace(connection.BaseURL) == "" {
		return errors.New("douyin client base_url is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(connection.BaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("douyin client base_url must be an absolute http or https URL")
	}
	if connection.Timeout.Duration <= 0 {
		return errors.New("douyin client timeout must be greater than zero")
	}
	return nil
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func max(value, low int) int {
	if value < low {
		return low
	}
	return value
}
