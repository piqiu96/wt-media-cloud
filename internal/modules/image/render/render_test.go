package render

import (
	"bytes"
	"context"
	"image/png"
	"strings"
	"testing"
)

func TestRenderActualChineseCoverAndOverflow(t *testing.T) {
	d := DefaultDocument(1080, 1440)
	data, issues, e := Render(context.Background(), d, "精彩实战指南", nil)
	if e != nil || len(issues) > 0 {
		t.Fatalf("render: %+v %v", issues, e)
	}
	image, e := png.Decode(bytes.NewReader(data))
	if e != nil || image.Bounds().Dx() != 1080 || image.Bounds().Dy() != 1440 {
		t.Fatalf("PNG: %v", e)
	}
	again, _, e := Render(context.Background(), d, "精彩实战指南", nil)
	if e != nil || !bytes.Equal(data, again) {
		t.Fatal("render is not deterministic")
	}
	d.Layers[0].MaxChars = 2
	if _, issues, e = Render(context.Background(), d, "不能截断标题", nil); e == nil || len(issues) == 0 {
		t.Fatal("overflow accepted")
	}
}
func TestHiddenTitleCountsAndDraftMayRemainIncomplete(t *testing.T) {
	d := DefaultDocument(1080, 1440)
	hidden := d.Layers[0]
	hidden.ID = "hidden"
	hidden.Visible = false
	d.Layers = append(d.Layers, hidden)
	if _, _, e := Render(context.Background(), d, "主标题", nil); e == nil {
		t.Fatal("duplicate hidden title accepted")
	}
}
func TestRenderRejectsMissingImageAndUnknownFont(t *testing.T) {
	d := DefaultDocument(1080, 1440)
	d.BackgroundMode = "business"
	if _, _, e := Render(context.Background(), d, "主标题", nil); e == nil {
		t.Fatal("missing business background accepted")
	}
	d.BackgroundMode = "solid"
	d.Layers[0].FontFamily = "not-installed"
	if _, _, e := Render(context.Background(), d, "主标题", nil); e == nil {
		t.Fatal("unknown font accepted")
	}
}

func TestCharacterLimitIsSeparateFromLayoutAndLocatesItsControl(t *testing.T) {
	d := DefaultDocument(1080, 1440)
	d.Layers[0].MaxChars = 2
	_, issues, err := Render(context.Background(), d, "三个字", nil)
	if err == nil || len(issues) != 1 || issues[0].Code != "max_chars" || issues[0].Field != "maxChars" || !strings.Contains(issues[0].Message, "3字") || !strings.Contains(issues[0].Message, "2字") {
		t.Fatalf("character limit was confused with layout: %+v %v", issues, err)
	}
	d.Layers[0].MaxChars = 0
	d.Layers[0].W = 10
	_, issues, err = Render(context.Background(), d, "三个字", nil)
	if err == nil || len(issues) == 0 || issues[0].Code != "layout" || issues[0].Field != "w" {
		t.Fatalf("layout did not locate text area: %+v %v", issues, err)
	}
}
