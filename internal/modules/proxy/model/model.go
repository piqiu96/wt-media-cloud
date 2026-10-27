// Package model owns Proxy domain values.
package model

import (
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type ProxyProtocol string

const (
	ProtocolHTTP   ProxyProtocol = "http"
	ProtocolHTTPS  ProxyProtocol = "https"
	ProtocolSOCKS5 ProxyProtocol = "socks5"
)

// BusinessStatus 是运营控制的启停开关。到期状态由 ExpiresAt 现算而不落库，
// 避免同一资源在一处显示已到期、另一处却仍可分配。
type BusinessStatus string

const (
	BizActive BusinessStatus = "active"
	BizPaused BusinessStatus = "paused"
)

// 检测结果区分失败原因。只有 CheckOK 的资源可分配；检测成功不会顺延到期时间。
const (
	CheckOK         = "ok"
	CheckConnFailed = "conn_failed"
	CheckAuthFailed = "auth_failed"
	CheckTimeout    = "timeout"
	CheckInvalid    = "invalid_params"
)

type ProxySourceType string

const (
	ProxySourceStatic ProxySourceType = "static"
	ProxySourceAPI    ProxySourceType = "api"
)

type ProxyConfig struct {
	ID                    string                `json:"id"`
	TeamID                sharedidentity.TeamID `json:"team_id"`
	Name                  string                `json:"name"`
	SourceType            ProxySourceType       `json:"source_type"`
	ProxyProtocol         ProxyProtocol         `json:"proxy_protocol"`
	Host                  string                `json:"host"`
	Port                  int                   `json:"port"`
	Username              string                `json:"username,omitempty"`
	Password              string                `json:"password,omitempty"`
	Region                string                `json:"region,omitempty"`
	Supplier              string                `json:"supplier,omitempty"`
	Tags                  []string              `json:"tags"`
	ExtractURL            string                `json:"-"`
	ExtractURLConfigured  bool                  `json:"extract_url_configured,omitempty"`
	ExpiresAt             *time.Time            `json:"expires_at,omitempty"`
	BusinessStatus        BusinessStatus        `json:"business_status"`
	MaxProfileCount       int                   `json:"max_profile_count"`
	LastCheckAt           *time.Time            `json:"last_check_at,omitempty"`
	LastCheckResult       string                `json:"last_check_result,omitempty"`
	ObservedExitIP        string                `json:"observed_exit_ip,omitempty"`
	AssignedProfileCount  int                   `json:"assigned_profile_count,omitempty"`
	RemainingProfileCount int                   `json:"remaining_profile_count,omitempty"`
	Remark                string                `json:"remark,omitempty"`
	CreatedAt             time.Time             `json:"created_at"`
	UpdatedAt             time.Time             `json:"updated_at"`
}

// Expired 判断资源是否已过到期时间。日期为空表示「未配置到期」，不是已到期。
func (p ProxyConfig) Expired(now time.Time) bool {
	return p.ExpiresAt != nil && !p.ExpiresAt.After(now)
}
