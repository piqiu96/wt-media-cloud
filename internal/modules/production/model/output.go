package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

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
