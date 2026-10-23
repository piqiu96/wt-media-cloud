// Package dto owns Profile Binding input contracts.
package dto

import "time"

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
	MainUserID string `json:"main_user_id"`
	// Agent 读取 BitBrowser 状态的时间。观察过期的扫描会被拒绝：其中的 Profile
	// 列表已不能证明本机当前打开着什么。
	ObservedAt time.Time      `json:"observed_at"`
	Profiles   []ProfileInput `json:"profiles"`
}

// BitAccountBindInput 是 Local Agent 读取正在运行的 BitBrowser 主账号后提交的事实。
// 没有其他写入路径：手工填写的 main_user_id 是断言，不是观察。
type BitAccountBindInput struct {
	MainUserID string    `json:"main_user_id"`
	ObservedAt time.Time `json:"observed_at"`
	// Rebind 表示用户已在界面确认观测到的主账号与已绑定账号不同。未携带时
	// Cloud 拒绝不一致且不写入任何状态。
	Rebind bool `json:"rebind"`
}
