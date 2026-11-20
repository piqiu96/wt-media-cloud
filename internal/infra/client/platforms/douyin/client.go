// Package douyin owns the external Douyin HTTP protocol.
package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	hertzerrors "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/wt-media/wt-media-cloud/internal/config"
	httpclient "github.com/wt-media/wt-media-cloud/pkg/clients/http"
)

// ErrUnavailable identifies a failed upstream response without exposing its body.
var ErrUnavailable = errors.New("douyin upstream unavailable")

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
	origin    string
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

// Initialize builds the Douyin client from its named Hertz transport and publishes it.
func Initialize(name string, credentials config.CredentialsConfig) (func() error, error) {
	transport := httpclient.Get(name)
	client := NewWithClient(transport.Origin(), credentials.Douyin, transport)
	resources.Lock()
	defer resources.Unlock()
	if resources.client != nil {
		return nil, errors.New("douyin client already initialized")
	}
	resources.client = client
	return Close, nil
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
func NewWithClient(origin string, credential config.DouyinCredentialConfig, transport *httpclient.Client) *Client {
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
		origin:    strings.TrimRight(origin, "/"),
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
	// The detail provider accepts canonical video pages by ID. Sending these
	// pages as shorturl is rejected; actual short links keep their own protocol.
	if u, err := url.Parse(source); err == nil && (u.Scheme == "https" || u.Scheme == "http") && (strings.EqualFold(u.Hostname(), "www.douyin.com") || strings.EqualFold(u.Hostname(), "douyin.com")) {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 2 && parts[0] == "video" {
			if id, err := strconv.ParseUint(parts[1], 10, 64); err == nil && id > 0 {
				source = parts[1]
			}
		}
	}
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
	started := time.Now()
	fail := func(reason string, status int, err error) error {
		// RequestContext supplies trace_id/request_id. Never log the credential
		// query, submitted content, provider body, or raw transport error here.
		hlog.CtxErrorf(ctx, "module=douyin path=%s failure=%s status=%d duration=%s", path, reason, status, time.Since(started))
		return err
	}
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	endpoint := c.origin + path + "?apiKey=" + url.QueryEscape(c.apiKey)

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
		var networkError net.Error
		if errors.Is(err, hertzerrors.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
			return fail("upstream_timeout", 0, fmt.Errorf("douyin request: %w", context.DeadlineExceeded))
		}
		if errors.Is(err, context.Canceled) {
			return fail("request_canceled", 0, context.Canceled)
		}
		return fail("transport_failed", 0, fmt.Errorf("%w: transport failed", ErrUnavailable))
	}
	if response.StatusCode() < 200 || response.StatusCode() >= 300 {
		if response.StatusCode() == 504 || response.StatusCode() == 408 {
			return fail("upstream_timeout", response.StatusCode(), fmt.Errorf("douyin HTTP status %d: %w", response.StatusCode(), context.DeadlineExceeded))
		}
		return fail("http_error", response.StatusCode(), fmt.Errorf("%w: HTTP status %d", ErrUnavailable, response.StatusCode()))
	}
	if err := json.Unmarshal(response.BodyBytes(), target); err != nil {
		return fail("invalid_response", response.StatusCode(), fmt.Errorf("%w: invalid JSON response", ErrUnavailable))
	}
	if !successValue((*target)["result"]) {
		return fail("provider_rejected", response.StatusCode(), fmt.Errorf("%w: API rejected request", ErrUnavailable))
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
