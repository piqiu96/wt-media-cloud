package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	jsonencode "github.com/goccy/go-json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ExecutionReport struct {
	SpaceCoverSHA256    string       `json:"space_cover_sha256,omitempty"`
	SceneParked         bool         `json:"scene_parked,omitempty"`
	PortraitCoverSHA256 string       `json:"portrait_cover_sha256,omitempty"`
	AttemptID           uint64       `json:"attempt_id,string"`
	Seq                 uint64       `json:"report_seq"`
	Stage               AttemptStage `json:"stage,omitempty"`
	PlatformUID         string       `json:"platform_uid,omitempty"`
	BitProfileID        string       `json:"bit_profile_id,omitempty"`
	VideoSHA256         string       `json:"video_sha256,omitempty"`
	CoverSHA256         string       `json:"cover_sha256,omitempty"`
}
type Completion struct {
	AttemptID  uint64          `json:"attempt_id,string"`
	Seq        uint64          `json:"report_seq"`
	ResultHash string          `json:"result_hash"`
	Result     json.RawMessage `json:"result"`
}
type ResultEvidence struct {
	SpaceCoverSHA256    string `json:"space_cover_sha256,omitempty"`
	PortraitCoverSHA256 string `json:"portrait_cover_sha256,omitempty"`
	PlatformUID         string `json:"platform_uid,omitempty"`
	BitProfileID        string `json:"bit_profile_id,omitempty"`
	VideoSHA256         string `json:"video_sha256,omitempty"`
	CoverSHA256         string `json:"cover_sha256,omitempty"`
	ReceiptID           string `json:"receipt_id,omitempty"`
	NotSubmitted        bool   `json:"not_submitted,omitempty"`
	SceneClosed         bool   `json:"scene_closed,omitempty"`
}
type ExecutionResult struct {
	Status         Status         `json:"status"`
	PlatformWorkID string         `json:"platform_work_id,omitempty"`
	WorkURL        string         `json:"work_url,omitempty"`
	ErrorCode      string         `json:"error_code,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	Evidence       ResultEvidence `json:"evidence"`
}

func ParseResult(raw json.RawMessage) (ExecutionResult, error) {
	var v ExecutionResult
	if len(raw) > 24576 || !utf8.Valid(raw) {
		return v, errors.New("invalid publication evidence")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&v); e != nil {
		return v, e
	}
	if decoder.Decode(new(any)) != io.EOF {
		return v, errors.New("trailing result JSON")
	}
	return v, nil
}

const ExecutionInputChanged = "PUBLICATION_INPUT_CHANGED"

type ExecutionFeedback struct {
	SceneParked   bool          `json:"scene_parked"`
	StopReason    string        `json:"stop_reason,omitempty"`
	TaskStatus    Status        `json:"task_status"`
	AttemptStatus AttemptStatus `json:"attempt_status"`
	AttemptID     uint64        `json:"attempt_id,string"`
	Seq           uint64        `json:"report_seq"`
	StopRequested bool          `json:"stop_requested"`
	AllowSubmit   bool          `json:"allow_submit"`
	ResultHash    *string       `json:"result_hash,omitempty"`
}

// Canonical JSON matches the Agent's persisted UTF-8 packet (Python sort_keys,
// ensure_ascii=False). No floating point or private runtime data is admitted.
func CompletionHash(seq uint64, raw json.RawMessage) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if !utf8.Valid(raw) {
		return "", errors.New("invalid UTF-8")
	}
	if e := decoder.Decode(&value); e != nil {
		return "", e
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", errors.New("trailing JSON")
	}
	value, e := integerJSON(value)
	if e != nil {
		return "", e
	}
	packet, e := jsonencode.MarshalWithOption(map[string]any{"report_seq": seq, "result": value}, jsonencode.DisableHTMLEscape(), jsonencode.DisableNormalizeUTF8())
	if e != nil {
		return "", e
	}
	digest := sha256.Sum256(packet)
	return hex.EncodeToString(digest[:]), nil
}

// Validate and normalize integers before the general JSON encoder handles keys,
// strings and containers. This preserves the persisted Agent packet semantics.
func integerJSON(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			normalized, e := integerJSON(item)
			if e != nil {
				return nil, e
			}
			v[key] = normalized
		}
	case []any:
		for i, item := range v {
			normalized, e := integerJSON(item)
			if e != nil {
				return nil, e
			}
			v[i] = normalized
		}
	case json.Number:
		text := string(v)
		if strings.ContainsAny(text, ".eE") {
			return nil, errors.New("evidence numbers must be integers")
		}
		if strings.HasPrefix(text, "-") {
			return strconv.ParseInt(text, 10, 64)
		}
		return strconv.ParseUint(text, 10, 64)
	}
	return value, nil
}
