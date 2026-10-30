package repository

import (
	"encoding/json"
	"errors"
)

// ErrLockTimeout 表示同一代理的另一次分配仍持有命名锁；调用方必须报可重试
// 冲突，而不是笼统的内部错误。
var ErrLockTimeout = errors.New("proxy assignment lock timed out")

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func splitTags(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}
