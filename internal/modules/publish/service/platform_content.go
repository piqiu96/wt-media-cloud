package service

import (
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

// Bilibili's upload text inputs enforce JavaScript/HTML UTF-16 lengths.
// Keep Baidu's established Unicode code-point rule unchanged.
func platformTextLength(platform, value string) int {
	if platform == "bilibili" {
		return len(utf16.Encode([]rune(value)))
	}
	return utf8.RuneCountInString(value)
}

func contentValidFor(platform string, c model.Content) bool {
	if platform == "baidu" {
		return c.Title == "" && c.Bilibili == nil && c.SpaceCoverFileID == 0 && c.SpaceCoverRender == nil &&
			strings.TrimSpace(c.Description) != "" && publicationTextLength(c.Description, c.Topics) <= 50 && c.CoverFileID > 0 && c.PortraitCoverFileID > 0 && c.Declaration == "无需声明" && utf8.RuneCountInString(c.Topics) <= 200 && c.Confirmed
	}
	if platform != "bilibili" || c.Bilibili == nil || !c.Confirmed || c.CoverFileID <= 0 || c.SpaceCoverFileID <= 0 || c.PortraitCoverFileID != 0 || c.PortraitCoverRender != nil || c.Topics != "" || c.Declaration != "" || strings.TrimSpace(c.Title) == "" || strings.ContainsAny(c.Title, "\r\n") || platformTextLength(platform, c.Title) > 80 || platformTextLength(platform, c.Description) > 2000 {
		return false
	}
	b := c.Bilibili
	if b.PartitionID <= 0 || strings.TrimSpace(b.PartitionName) == "" || utf8.RuneCountInString(b.PartitionName) > 100 || len(b.Tags) < 1 || len(b.Tags) > 10 || b.Visibility != "public" && b.Visibility != "private" {
		return false
	}
	seen := map[string]bool{}
	for _, tag := range b.Tags {
		if tag == "" || strings.TrimSpace(tag) != tag || strings.ContainsAny(tag, ",，\r\n") || platformTextLength(platform, tag) > 20 || seen[tag] {
			return false
		}
		seen[tag] = true
	}
	if !slices.Contains(bilibiliStatements, b.Statement) {
		return false
	}
	if platformTextLength(platform, b.Source) > 200 {
		return false
	}
	if b.Statement == "内容为转载" {
		return strings.TrimSpace(b.Source) != "" && !b.RightsReserved
	}
	return b.Source == ""
}
