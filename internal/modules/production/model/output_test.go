package model

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestOutputNamesAreStableSafeAndBounded(t *testing.T) {
	name := ComposeOutputName("素材标题", "短版")
	if name != "素材标题 · 短版" {
		t.Fatal(name)
	}
	file := ComposeOutputFileName("../CON\\secret:\x00片", 42, ".mp4")
	if strings.ContainsAny(file, "/\\:\x00") || !strings.HasSuffix(file, "-42.mp4") {
		t.Fatal(file)
	}
	if len(ComposeOutputFileName(strings.Repeat("片", 255), 42, ".mp4")) > 255 {
		t.Fatal("oversized filename")
	}
	if !utf8.ValidString(ComposeOutputFileName(strings.Repeat("片", 255), 9223372036854775807, ".mp4")) {
		t.Fatal("split rune")
	}
}
