package model

import (
	"encoding/json"
	"strings"
)

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
		return "执行异常，请联系高级运营或管理员查看诊断摘要"
	}
}

func (task ComposeTask) MarshalJSON() ([]byte, error) {
	type projection ComposeTask
	task.ErrorMessage = safeComposeErrorMessage(task.ErrorCode, task.ErrorMessage)
	task.ConfigSnapshot = safeComposeSnapshot(task.ConfigSnapshot)
	return json.Marshal(projection(task))
}

func safeComposeSnapshot(snapshot json.RawMessage) json.RawMessage {
	if len(snapshot) == 0 {
		return snapshot
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(snapshot, &fields); err != nil || fields == nil {
		return json.RawMessage(`{}`)
	}
	if _, exists := fields["guide_video_url"]; !exists {
		return snapshot
	}
	fields["guide_video_url"] = json.RawMessage(`"（下载地址已隐藏）"`)
	safe, err := json.Marshal(fields)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return safe
}

// composeStepLabels is the only vocabulary a step may be projected with. An
// entry carries its key from the server, so a value that is not in this table
// never belonged to a step the server recorded.
var composeStepLabels = map[string]string{
	ComposeStepValidateInput: "校验任务参数",
	ComposeStepPrepareInput:  "准备源素材",
	ComposeStepFetchGuide:    "下载引导视频",
	ComposeStepCut:           "视频合成",
	ComposeStepUpload:        "上传并校验成片",
	ComposeStepCommit:        "保存成片记录",
}

// ComposeStepLabel returns the operator-facing label of a step key, or "" when
// the key is not a step this server writes.
func ComposeStepLabel(step string) string { return composeStepLabels[step] }

// SafeComposeErrorLabel is safeComposeErrorMessage for a bare error code, for
// callers that only have the code.
func SafeComposeErrorLabel(code string) string { return safeComposeErrorMessage(code, "") }

// Normalize rebuilds the fields the server owns: the label, the failure label
// from the masking table and the metric allowlist. Outcome, timestamps and the
// error code are the caller's facts and stay as recorded.
func (step ComposeTaskAttemptStep) Normalize() ComposeTaskAttemptStep {
	step.Label = ComposeStepLabel(step.Step)
	if step.DurationMS < 0 {
		step.DurationMS = 0
	}
	if step.Outcome == ComposeStepOutcomeFailed {
		step.ErrorLabel = SafeComposeErrorLabel(step.ErrorCode)
	} else {
		step.ErrorCode = ""
		step.ErrorLabel = ""
	}
	metrics := ComposeAttemptStepMetrics{}
	if step.Metrics.SizeBytes != nil && *step.Metrics.SizeBytes > 0 {
		metrics.SizeBytes = step.Metrics.SizeBytes
	}
	if step.Metrics.OutputDurationMS != nil && *step.Metrics.OutputDurationMS > 0 {
		metrics.OutputDurationMS = step.Metrics.OutputDurationMS
	}
	if step.Metrics.OutputID != nil && *step.Metrics.OutputID > 0 {
		metrics.OutputID = step.Metrics.OutputID
	}
	if isComposeDigest(step.Metrics.SHA256) {
		metrics.SHA256 = strings.ToLower(step.Metrics.SHA256)
	}
	step.Metrics = metrics
	return step
}

func isComposeDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		switch {
		case char >= '0' && char <= '9':
		case char >= 'a' && char <= 'f':
		case char >= 'A' && char <= 'F':
		default:
			return false
		}
	}
	return true
}

// EncodeComposeAttemptStep is the write side: normalize, then serialize the one
// entry the worker is about to append.
func EncodeComposeAttemptStep(step ComposeTaskAttemptStep) ([]byte, error) {
	return json.Marshal(step.Normalize())
}

// DecodeComposeAttemptSteps reads the steps_json column back. A NULL column and
// an empty array are the same fact — an attempt with nothing recorded yet —
// and both decode to an empty list.
func DecodeComposeAttemptSteps(raw []byte) ([]ComposeTaskAttemptStep, error) {
	if len(raw) == 0 {
		return []ComposeTaskAttemptStep{}, nil
	}
	steps := []ComposeTaskAttemptStep{}
	if err := json.Unmarshal(raw, &steps); err != nil {
		return nil, err
	}
	if steps == nil {
		steps = []ComposeTaskAttemptStep{}
	}
	return steps, nil
}

// projectComposeAttemptSteps rebuilds the wire shape instead of forwarding the
// column: an unknown key or outcome is dropped, and every label is recomputed
// (a stored string is never trusted). A new JSON field would otherwise publish
// whatever the column holds verbatim — an address or a local path included, if
// one ever reached it.
func projectComposeAttemptSteps(steps []ComposeTaskAttemptStep) []ComposeTaskAttemptStep {
	projected := make([]ComposeTaskAttemptStep, 0, len(steps))
	for _, step := range steps {
		if ComposeStepLabel(step.Step) == "" {
			continue
		}
		if step.Outcome != ComposeStepOutcomeSuccess && step.Outcome != ComposeStepOutcomeFailed {
			continue
		}
		projected = append(projected, step.Normalize())
	}
	return projected
}

func (attempt ComposeTaskAttempt) MarshalJSON() ([]byte, error) {
	type projection ComposeTaskAttempt
	attempt.ErrorMessage = safeComposeErrorMessage(attempt.ErrorCode, attempt.ErrorMessage)
	attempt.Steps = projectComposeAttemptSteps(attempt.Steps)
	return json.Marshal(projection(attempt))
}
