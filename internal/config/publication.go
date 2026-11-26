package config

import (
	"errors"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func (c PublicationPlatformsConfig) Validate() error {
	d := c.Bilibili
	if d.PartitionID < 0 || (d.PartitionID == 0) != (d.PartitionName == "") || strings.TrimSpace(d.PartitionName) != d.PartitionName || utf8.RuneCountInString(d.PartitionName) > 100 || d.Visibility != "" && d.Visibility != "public" && d.Visibility != "private" || len(d.Tags) > 10 {
		return errors.New("Bilibili publication defaults invalid")
	}
	seen := map[string]bool{}
	for _, tag := range d.Tags {
		if tag == "" || strings.TrimSpace(tag) != tag || strings.ContainsAny(tag, ",，\r\n") || len(utf16.Encode([]rune(tag))) > 20 || seen[tag] {
			return errors.New("Bilibili publication default tags invalid")
		}
		seen[tag] = true
	}
	return nil
}
