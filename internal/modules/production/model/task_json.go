package model

import "encoding/json"

// Task errors may contain FFmpeg commands, local paths or signed storage URLs.
// Keep the detailed message in the database for operations, but expose only a
// fixed category label in task and attempt API projections.
func safeComposeErrorMessage(code, message string) string {
	if code == "" && message == "" {
		return ""
	}
	switch code {
	case "execution_uncertain":
		return "结果不明确"
	case "storage_error":
		return "存储暂时失败"
	case "task_timeout":
		return "任务超时"
	case "input_missing":
		return "源文件缺失"
	case "input_invalid":
		return "素材校验失败"
	case "template_unavailable":
		return "模板不可用"
	case "execution_failed":
		return "执行失败"
	default:
		return "执行异常，请联系管理员查看诊断摘要"
	}
}

func (task ComposeTask) MarshalJSON() ([]byte, error) {
	type projection ComposeTask
	task.ErrorMessage = safeComposeErrorMessage(task.ErrorCode, task.ErrorMessage)
	return json.Marshal(projection(task))
}

func (attempt ComposeTaskAttempt) MarshalJSON() ([]byte, error) {
	type projection ComposeTaskAttempt
	attempt.ErrorMessage = safeComposeErrorMessage(attempt.ErrorCode, attempt.ErrorMessage)
	return json.Marshal(projection(attempt))
}
