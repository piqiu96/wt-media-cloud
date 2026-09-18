// Package dto owns Proxy input and query contracts.
package dto

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
)

type CreateProxyInput struct {
	SourceType      model.ProxySourceType `json:"source_type"`
	ProxyProtocol   model.ProxyProtocol   `json:"proxy_protocol"`
	Host            string                `json:"host"`
	Port            int                   `json:"port"`
	Username        string                `json:"username"`
	Password        string                `json:"password"`
	Region          string                `json:"region"`
	Supplier        string                `json:"supplier"`
	ExtractURL      string                `json:"extract_url"`
	ExpiresAt       *time.Time            `json:"expires_at"`
	Remark          string                `json:"remark"`
	MaxProfileCount int                   `json:"max_profile_count"`
}

type BulkImportRow struct {
	Raw    string            `json:"raw"`
	Parsed *CreateProxyInput `json:"parsed,omitempty"`
	Error  string            `json:"error,omitempty"`
}

type ProxyFilter struct {
	Platform       string `json:"platform"`
	Supplier       string `json:"supplier"`
	BusinessStatus string `json:"business_status"`
	Region         string `json:"region"`
	Search         string `json:"search"`
	Limit          int    `json:"limit"`
	Offset         int    `json:"offset"`
}

type ProxyCheckInput struct {
	ProxyID       string              `json:"proxy_id,omitempty"`
	ProxyProtocol model.ProxyProtocol `json:"proxy_protocol"`
	Host          string              `json:"host"`
	Port          int                 `json:"port"`
}

type ProxyCheckResult struct {
	Connectivity string `json:"connectivity"`
}

type ProxyExtractionInput struct {
	ExtractURL    string              `json:"extract_url"`
	ProxyProtocol model.ProxyProtocol `json:"proxy_protocol"`
}

type ProxyExtractionResult struct {
	ProxyProtocol model.ProxyProtocol `json:"proxy_protocol"`
	Host          string              `json:"host"`
	Port          int                 `json:"port"`
	Username      string              `json:"username,omitempty"`
	Password      string              `json:"password,omitempty"`
}

type ProxyMutationInput struct {
	Operation     string              `json:"operation,omitempty"`
	ProfileID     string              `json:"profile_id"`
	ProxyProtocol model.ProxyProtocol `json:"proxy_protocol"`
	Host          string              `json:"host"`
	Port          int                 `json:"port"`
	Username      string              `json:"username,omitempty"`
	Password      string              `json:"password,omitempty"`
}

type ProxyMutationResult struct {
	Operation     string              `json:"operation,omitempty"`
	ProfileID     string              `json:"profile_id"`
	ProxyProtocol model.ProxyProtocol `json:"proxy_protocol"`
	Host          string              `json:"host"`
	Port          int                 `json:"port"`
	Readback      bool                `json:"readback"`
}
