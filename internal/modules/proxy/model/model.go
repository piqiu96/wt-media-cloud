// Package model owns Proxy domain values.
package model

import (
	"time"

	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type Protocol string

const (
	ProtocolHTTP   Protocol = "http"
	ProtocolHTTPS  Protocol = "https"
	ProtocolSOCKS5 Protocol = "socks5"
)

// 检测结果区分失败原因。只有 CheckOK 的资源可分配；检测成功不会顺延到期时间。
const (
	CheckOK         = "ok"
	CheckConnFailed = "conn_failed"
	CheckAuthFailed = "auth_failed"
	CheckTimeout    = "timeout"
	CheckInvalid    = "invalid_params"
)

type ProxySource string

const (
	ProxySourceStatic ProxySource = "static"
	ProxySourceAPI    ProxySource = "api"
)

type Proxy struct {
	ID                    string                `json:"id"`
	TeamID                sharedidentity.TeamID `json:"team_id"`
	Name                  string                `json:"name"`
	Source                ProxySource           `json:"source"`
	Protocol              Protocol              `json:"protocol"`
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
	Enabled               bool                  `json:"enabled"`
	MaxProfiles           int                   `json:"max_profiles"`
	LastCheckAt           *time.Time            `json:"last_check_at,omitempty"`
	LastCheckResult       string                `json:"last_check_result,omitempty"`
	LastCheckMessage      string                `json:"last_check_message,omitempty"`
	AssignedProfileCount  int                   `json:"assigned_profile_count"`
	ReservedProfileCount  int                   `json:"reserved_profile_count"`
	RemainingProfileCount int                   `json:"remaining_profile_count"`
	Remark                string                `json:"remark,omitempty"`
	CreatedAt             time.Time             `json:"created_at"`
	UpdatedAt             time.Time             `json:"updated_at"`
}

// Expired 判断资源是否已过到期时间。日期为空表示「未配置到期」，不是已到期。
func (p Proxy) Expired(now time.Time) bool {
	return p.ExpiresAt != nil && !p.ExpiresAt.After(now)
}
