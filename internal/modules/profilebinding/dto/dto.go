// Package dto owns Profile Binding input contracts.
package dto

import "time"

type ProfileInput struct {
	BitProfileID     string `json:"bit_profile_id"`
	BitMainUserID    string `json:"bit_main_user_id"`
	BitUserID        string `json:"bit_user_id"`
	BitName          string `json:"bit_name"`
	BitSeq           int    `json:"bit_seq"`
	BitGroupID       string `json:"bit_group_id"`
	BitGroupName     string `json:"bit_group_name"`
	BitProfileStatus string `json:"bit_profile_status"`
	BitUpdatedAt     string `json:"bit_updated_at"`
	BitProxyType     string `json:"bit_proxy_type"`
	BitProxyHost     string `json:"bit_proxy_host"`
	BitProxyPort     int    `json:"bit_proxy_port"`
	BitRemark        string `json:"bit_remark"`
}

type SnapshotInput struct {
	BitMainUserID string `json:"bit_main_user_id"`
	// Agent 读取 BitBrowser 状态的时间。观察过期的扫描会被拒绝：其中的 Profile
	// 列表已不能证明本机当前打开着什么。
	ObservedAt time.Time      `json:"observed_at"`
	Profiles   []ProfileInput `json:"profiles"`
}

// BitAccountBindInput 是 Local Agent 读取正在运行的 BitBrowser 主账号后提交的事实。
// 没有其他写入路径：手工填写的 main_user_id 是断言，不是观察。
type BitAccountBindInput struct {
	BitMainUserID string    `json:"bit_main_user_id"`
	ObservedAt    time.Time `json:"observed_at"`
	// Rebind 表示用户已在界面确认观测到的主账号与已绑定账号不同。未携带时
	// Cloud 拒绝不一致且不写入任何状态。
	Rebind bool `json:"rebind"`
}
