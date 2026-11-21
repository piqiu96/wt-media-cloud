package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const OutputPublishArchived = "archived"

type ComposeOutputVideo struct {
	Status    string     `json:"status"`
	Reason    string     `json:"reason"`
	CheckedAt *time.Time `json:"checked_at"`
}
type ComposeOutputActions struct {
	Download       bool   `json:"download"`
	Remake         bool   `json:"remake"`
	DownloadReason string `json:"download_reason"`
	RemakeReason   string `json:"remake_reason"`
}

func ComposeOutputName(title, setting string) string {
	shorten := func(s string, n int) string {
		r := []rune(strings.TrimSpace(s))
		if len(r) > n {
			return string(r[:n]) + "…"
		}
		return string(r)
	}
	title = strings.TrimSpace(title)
	setting = strings.TrimSpace(setting)
	if title == "" || setting == "" {
		return ""
	}
	return shorten(title, 120) + " · " + shorten(setting, 120)
}
func ComposeOutputFileName(name string, id int64, extension string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name)
	clean = strings.Trim(clean, " .-")
	if clean == "" {
		clean = "成片"
	}
	// Reserve bytes for the unique suffix; filesystem limits apply to UTF-8 bytes.
	if extension != ".mp4" {
		extension = ".mp4"
	}
	suffix := fmt.Sprintf("-%d%s", id, extension)
	budget := 255 - len(suffix)
	for len(clean) > budget {
		_, n := utf8.DecodeLastRuneInString(clean)
		clean = clean[:len(clean)-n]
	}

	return fmt.Sprintf("%s-%d%s", clean, id, extension)
}

// SafeComposeSnapshot applies the same read projection as task APIs, without
// altering the execution snapshot retained in the database.
func SafeComposeSnapshot(raw json.RawMessage) json.RawMessage { return safeComposeSnapshot(raw) }

// ThumbnailFileID reads only the provider-owned reference stored with media facts.
func (o CompositeOutput) ThumbnailFileID() (int64, error) {
	if len(o.Media) == 0 {
		return 0, nil
	}
	var facts struct {
		ThumbnailFileID string `json:"thumbnail_file_id"`
	}
	if err := json.Unmarshal(o.Media, &facts); err != nil {
		return 0, err
	}
	if facts.ThumbnailFileID == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(facts.ThumbnailFileID, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != facts.ThumbnailFileID {
		return 0, fmt.Errorf("invalid output thumbnail reference")
	}
	return id, nil
}

type ComposeOutputThumbnail struct {
	FileID       int64  `json:"file_id,string"`
	ResourcePath string `json:"resource_path"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}
