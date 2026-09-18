// Package model owns Proxy domain values.
package model

import "time"

type ProxyProtocol string

const (
	ProtocolHTTP   ProxyProtocol = "http"
	ProtocolHTTPS  ProxyProtocol = "https"
	ProtocolSOCKS5 ProxyProtocol = "socks5"
)

type BusinessStatus string

const (
	BizActive  BusinessStatus = "active"
	BizPaused  BusinessStatus = "paused"
	BizExpired BusinessStatus = "expired"
)

type ProxySourceType string

const (
	ProxySourceStatic ProxySourceType = "static"
	ProxySourceAPI    ProxySourceType = "api"
)

type ProxyConfig struct {
	ID                    string          `json:"id"`
	SourceType            ProxySourceType `json:"source_type"`
	ProxyProtocol         ProxyProtocol   `json:"proxy_protocol"`
	Host                  string          `json:"host"`
	Port                  int             `json:"port"`
	Username              string          `json:"username,omitempty"`
	Password              string          `json:"password,omitempty"`
	Region                string          `json:"region,omitempty"`
	Supplier              string          `json:"supplier,omitempty"`
	ExtractURL            string          `json:"-"`
	ExtractURLConfigured  bool            `json:"extract_url_configured,omitempty"`
	ExpiresAt             *time.Time      `json:"expires_at,omitempty"`
	BusinessStatus        BusinessStatus  `json:"business_status"`
	MaxProfileCount       int             `json:"max_profile_count"`
	LastCheckAt           *time.Time      `json:"last_check_at,omitempty"`
	LastCheckResult       string          `json:"last_check_result,omitempty"`
	ObservedExitIP        string          `json:"observed_exit_ip,omitempty"`
	AssignedProfileCount  int             `json:"assigned_profile_count,omitempty"`
	RemainingProfileCount int             `json:"remaining_profile_count,omitempty"`
	Remark                string          `json:"remark,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}
