// Package douyin owns the external Douyin HTTP protocol.
package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
)

type SearchRequest struct {
	Keyword string
	Limit   int
	Offset  int
}

type FindAuthorRequest struct {
	Author string
	Limit  int
	Offset int
}

type FetchByURLRequest struct {
	URL string
}

type Client struct {
	baseURL    string
	apiKey     string
	cookie     string
	headers    map[string]string
	httpClient *http.Client
	attempts   int
	interval   time.Duration
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
	client := NewWithHTTPClient(connection, credential, nil)
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
	client.httpClient.CloseIdleConnections()
	return nil
}

// NewWithHTTPClient builds a testable Douyin client with an injected HTTP transport.
func NewWithHTTPClient(connection config.ClientConfig, credential config.DouyinCredentialConfig, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	httpClient.Timeout = connection.Timeout.Duration
	attempts := connection.Retry.Attempts
	if attempts < 1 {
		attempts = 1
	}
	headers := make(map[string]string, len(credential.Headers))
	for key, value := range credential.Headers {
		if strings.TrimSpace(key) != "" {
			headers[key] = value
		}
	}
	return &Client{
		baseURL:    buildBaseURL(connection),
		apiKey:     credential.APIKey,
		cookie:     credential.Cookie,
		headers:    headers,
		httpClient: httpClient,
		attempts:   attempts,
		interval:   connection.Retry.Interval.Duration,
	}
}

// Configured reports whether the separately loaded Douyin API key is present.
func (c *Client) Configured() bool {
	return c != nil && strings.TrimSpace(c.apiKey) != ""
}

// Search requests keyword search results from the public Douyin data API.
func (c *Client) Search(ctx context.Context, request SearchRequest) (map[string]any, error) {
	values := map[string]string{
		"keywords":        request.Keyword,
		"limit":           strconv.Itoa(clamp(request.Limit, 1, 30)),
		"offset":          strconv.Itoa(max(request.Offset, 0)),
		"sort_type":       "0",
		"content_type":    "1",
		"publish_time":    "0",
		"filter_duration": "0",
	}
	if c.cookie != "" {
		values["ck"] = c.cookie
	}
	var payload map[string]any
	err := c.postForm(ctx, "/dyRank", values, &payload)
	return payload, err
}

// FindAuthor requests an author's posts from the public Douyin data API.
func (c *Client) FindAuthor(ctx context.Context, request FindAuthorRequest) (map[string]any, error) {
	values := map[string]string{
		"uid":      request.Author,
		"nickname": request.Author,
		"limit":    strconv.Itoa(clamp(request.Limit, 1, 30)),
		"offset":   strconv.Itoa(max(request.Offset, 0)),
	}
	var payload map[string]any
	err := c.postForm(ctx, "/dyUser/detail", values, &payload)
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
	return doRequest(ctx, c.httpClient, c.attempts, c.interval, func() (*http.Request, error) {
		values := make(url.Values, len(fields))
		for key, value := range fields {
			values.Set(key, value)
		}
		endpoint := c.baseURL + path + "?apiKey=" + url.QueryEscape(c.apiKey)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("User-Agent", "WT-Media-Cloud/1")
		for key, value := range c.headers {
			request.Header.Set(key, value)
		}
		if c.cookie != "" {
			request.Header.Set("Cookie", c.cookie)
		}
		return request, nil
	}, func(response *http.Response) error {
		if err := json.NewDecoder(response.Body).Decode(target); err != nil {
			return fmt.Errorf("decode douyin response: %w", err)
		}
		if !successValue((*target)["result"]) {
			return errors.New("douyin API rejected request")
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
			return fmt.Errorf("douyin http status %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
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
	if connection.Scheme != "http" && connection.Scheme != "https" {
		return errors.New("douyin client scheme must be http or https")
	}
	if strings.TrimSpace(connection.Host) == "" {
		return errors.New("douyin client host is required")
	}
	if connection.Port <= 0 || connection.Port > 65535 {
		return errors.New("douyin client port is invalid")
	}
	if connection.Timeout.Duration <= 0 {
		return errors.New("douyin client timeout must be greater than zero")
	}
	if connection.Retry.Attempts < 1 {
		return errors.New("douyin client retry attempts must be greater than zero")
	}
	if connection.Retry.Interval.Duration < 0 {
		return errors.New("douyin client retry interval cannot be negative")
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
