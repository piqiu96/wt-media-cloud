// Package dto owns Proxy input and query contracts.
package dto

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type CreateProxyInput struct {
	// TeamID 由管理员指定；运营和高级运营固定落在本人团队，处理器会覆盖此值。
	TeamID      *sharedidentity.TeamID `json:"team_id,omitempty"`
	Name        string                 `json:"name"`
	Source      model.ProxySource      `json:"source"`
	Protocol    model.Protocol         `json:"protocol"`
	Host        string                 `json:"host"`
	Port        int                    `json:"port"`
	Username    string                 `json:"username"`
	Password    string                 `json:"password"`
	Region      string                 `json:"region"`
	Supplier    string                 `json:"supplier"`
	Tags        []string               `json:"tags"`
	ExtractURL  string                 `json:"extract_url"`
	ExpiresAt   *time.Time             `json:"expires_at"`
	Enabled     *bool                  `json:"enabled"`
	Remark      string                 `json:"remark"`
	MaxProfiles int                    `json:"max_profiles"`
}

type ProfileProxyOperationInput struct {
	Action  string `json:"action"`
	ProxyID string `json:"proxy_id"`
}

type ProfileProxyOperationResultInput struct {
	Action  string `json:"action"`
	ProxyID string `json:"proxy_id"`
	Outcome string `json:"outcome"`
	Message string `json:"message"`
}

type ProfileProxyOperationStart struct {
	Action        string         `json:"action"`
	ProfileID     string         `json:"profile_id"`
	BitProfileID  string         `json:"bit_profile_id"`
	ProxyID       string         `json:"proxy_id,omitempty"`
	Protocol      model.Protocol `json:"protocol,omitempty"`
	Host          string         `json:"host,omitempty"`
	Port          int            `json:"port,omitempty"`
	Username      string         `json:"username,omitempty"`
	Password      string         `json:"password,omitempty"`
	AgentRequired bool           `json:"agent_required"`
}

type BulkImportRow struct {
	Line      int               `json:"line"`
	Raw       string            `json:"raw"`
	Parsed    *CreateProxyInput `json:"parsed,omitempty"`
	Error     string            `json:"error,omitempty"`
	Duplicate bool              `json:"duplicate"`
	// DuplicateReason 指向占用同一规范化连接键的已有资源或更早的行。
	DuplicateReason string `json:"duplicate_reason,omitempty"`
}

// ImportRowInput 是运营提交的一行预览。未勾选的行也会提交，便于结果对账到
// 每一条原始输入。
type ImportRowInput struct {
	Line     int    `json:"line"`
	Raw      string `json:"raw"`
	Selected bool   `json:"selected"`
}

type ImportRowResult struct {
	Line    int    `json:"line"`
	Raw     string `json:"raw"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	ProxyID string `json:"proxy_id,omitempty"`
}

const (
	ImportStatusImported = "imported"
	ImportStatusFailed   = "failed"
	ImportStatusSkipped  = "skipped"
)

type ImportResult struct {
	Total    int               `json:"total"`
	Imported int               `json:"imported"`
	Failed   int               `json:"failed"`
	Skipped  int               `json:"skipped"`
	Rows     []ImportRowResult `json:"rows"`
}

type ProxyFilter struct {
	TeamID   *sharedidentity.TeamID `json:"team_id"`
	Platform string                 `json:"platform"`
	Supplier string                 `json:"supplier"`
	Enabled  *bool                  `json:"enabled"`
	Region   string                 `json:"region"`
	Search   string                 `json:"search"`
	Limit    int                    `json:"limit"`
	Offset   int                    `json:"offset"`
}

type ProxyCheckInput struct {
	ProxyID  string         `json:"proxy_id,omitempty"`
	Protocol model.Protocol `json:"protocol"`
	Host     string         `json:"host"`
	Port     int            `json:"port"`
	Username string         `json:"username,omitempty"`
	Password string         `json:"password,omitempty"`
}

type ProxyCheckResult struct {
	Connectivity string `json:"connectivity"`
	Reason       string `json:"reason,omitempty"`
}

type ProxyExtractionInput struct {
	ExtractURL string         `json:"extract_url"`
	Protocol   model.Protocol `json:"protocol"`
}

type ProxyExtractionResult struct {
	Protocol model.Protocol `json:"protocol"`
	Host     string         `json:"host"`
	Port     int            `json:"port"`
	Username string         `json:"username,omitempty"`
	Password string         `json:"password,omitempty"`
}

type ProxyMutationInput struct {
	Operation string         `json:"operation,omitempty"`
	ProfileID string         `json:"profile_id"`
	Protocol  model.Protocol `json:"protocol"`
	Host      string         `json:"host"`
	Port      int            `json:"port"`
	Username  string         `json:"username,omitempty"`
	Password  string         `json:"password,omitempty"`
}

type ProxyMutationResult struct {
	Operation string         `json:"operation,omitempty"`
	ProfileID string         `json:"profile_id"`
	Protocol  model.Protocol `json:"protocol"`
	Host      string         `json:"host"`
	Port      int            `json:"port"`
	Readback  bool           `json:"readback"`
}
