package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ExecutionReport struct {
	AttemptID    uint64 `json:"attempt_id,string"`
	Seq          uint64 `json:"report_seq"`
	Stage        string `json:"stage"`
	PlatformUID  string `json:"platform_uid,omitempty"`
	BitProfileID string `json:"bit_profile_id,omitempty"`
	VideoSHA256  string `json:"video_sha256,omitempty"`
	CoverSHA256  string `json:"cover_sha256,omitempty"`
}
type Completion struct {
	AttemptID  uint64          `json:"attempt_id,string"`
	Seq        uint64          `json:"report_seq"`
	ResultHash string          `json:"result_hash"`
	Result     json.RawMessage `json:"result"`
}
type ResultEvidence struct {
	PlatformUID  string `json:"platform_uid,omitempty"`
	BitProfileID string `json:"bit_profile_id,omitempty"`
	VideoSHA256  string `json:"video_sha256,omitempty"`
	CoverSHA256  string `json:"cover_sha256,omitempty"`
	ReceiptID    string `json:"receipt_id,omitempty"`
	NotSubmitted bool   `json:"not_submitted,omitempty"`
	SceneClosed  bool   `json:"scene_closed,omitempty"`
}
type ExecutionResult struct {
	Status         string         `json:"status"`
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

type ExecutionFeedback struct {
	TaskStatus    string  `json:"task_status"`
	AttemptStatus string  `json:"attempt_status"`
	AttemptID     uint64  `json:"attempt_id,string"`
	Seq           uint64  `json:"report_seq"`
	StopRequested bool    `json:"stop_requested"`
	AllowSubmit   bool    `json:"allow_submit"`
	ResultHash    *string `json:"result_hash,omitempty"`
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
	var out strings.Builder
	if e := canonical(&out, map[string]any{"report_seq": seq, "result": value}); e != nil {
		return "", e
	}
	digest := sha256.Sum256([]byte(out.String()))
	return hex.EncodeToString(digest[:]), nil
}
func canonical(out *strings.Builder, value any) error {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			canonicalString(out, k)
			out.WriteByte(':')
			if e := canonical(out, v[k]); e != nil {
				return e
			}
		}
		out.WriteByte('}')
	case []any:
		out.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				out.WriteByte(',')
			}
			if e := canonical(out, item); e != nil {
				return e
			}
		}
		out.WriteByte(']')
	case string:
		canonicalString(out, v)
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case nil:
		out.WriteString("null")
	case uint64:
		out.WriteString(strconv.FormatUint(v, 10))
	case json.Number:
		text := string(v)
		if strings.ContainsAny(text, ".eE") {
			return errors.New("evidence numbers must be integers")
		}
		if strings.HasPrefix(text, "-") {
			n, e := strconv.ParseInt(text, 10, 64)
			if e != nil {
				return e
			}
			out.WriteString(strconv.FormatInt(n, 10))
		} else {
			n, e := strconv.ParseUint(text, 10, 64)
			if e != nil {
				return e
			}
			out.WriteString(strconv.FormatUint(n, 10))
		}
	default:
		return errors.New("unsupported evidence value")
	}
	return nil
}
func canonicalString(out *strings.Builder, value string) {
	out.WriteByte('"')
	for _, c := range value {
		switch c {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if c < 32 {
				out.WriteString(`\u`)
				out.WriteString(strings.Repeat("0", 4-len(strconv.FormatInt(int64(c), 16))))
				out.WriteString(strconv.FormatInt(int64(c), 16))
			} else {
				out.WriteRune(c)
			}
		}
	}
	out.WriteByte('"')
}
