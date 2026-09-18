// Package dto owns Profile Binding input contracts.
package dto

type ProfileInput struct {
	BitProfileID  string `json:"bit_profile_id"`
	MainUserID    string `json:"main_user_id"`
	ProfileUserID string `json:"profile_user_id"`
	Name          string `json:"name"`
	Seq           int    `json:"seq"`
	GroupID       string `json:"group_id"`
	GroupName     string `json:"group_name"`
	BitStatus     string `json:"bit_status"`
	BitUpdatedAt  string `json:"bit_updated_at"`
	ProxyType     string `json:"proxy_type"`
	ProxyHost     string `json:"proxy_host"`
	ProxyPort     int    `json:"proxy_port"`
	Remark        string `json:"remark"`
}

type SnapshotInput struct {
	MainUserID string         `json:"main_user_id"`
	Profiles   []ProfileInput `json:"profiles"`
	NodeID     string         `json:"node_id,omitempty"`
}

type MainIdentityInput struct {
	MainUserID string `json:"main_user_id"`
}
