// Package render owns cover artwork and uses the same bounded document format as
// the editor. It never fetches URLs: the business service supplies decoded images.
package render

import (
	"fmt"
	"math"
	"strings"
)

const Version = "noto-cjk-20261121.1"
const ShortSample = "精彩实战指南"
const LongSample = "新手进阶必看：从基础操作到实战技巧的完整攻略"

type Document struct {
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Background     string  `json:"background"`
	BackgroundMode string  `json:"backgroundMode"`
	Image          string  `json:"image"`
	ImageAssetID   uint64  `json:"imageAssetId,string,omitempty"`
	ImageFit       string  `json:"imageFit"`
	ImageAnchor    string  `json:"imageAnchor"`
	Safe           float64 `json:"safe"`
	Layers         []Layer `json:"layers"`
}
type Layer struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Slot        string   `json:"slot"`
	Name        string   `json:"name"`
	Text        string   `json:"text"`
	X           float64  `json:"x"`
	Y           float64  `json:"y"`
	W           float64  `json:"w"`
	H           float64  `json:"h"`
	FontSize    float64  `json:"fontSize"`
	MinFont     float64  `json:"minFont"`
	FontFamily  string   `json:"fontFamily"`
	Weight      string   `json:"weight"`
	Color       string   `json:"color"`
	Align       string   `json:"align"`
	LineHeight  float64  `json:"lineHeight"`
	Spacing     float64  `json:"spacing"`
	MaxLines    int      `json:"maxLines"`
	MaxChars    int      `json:"maxChars"`
	Wrap        bool     `json:"wrap"`
	AutoShrink  bool     `json:"autoShrink"`
	Stroke      float64  `json:"stroke"`
	StrokeColor string   `json:"strokeColor"`
	Shadow      bool     `json:"shadow"`
	Opacity     *float64 `json:"opacity,omitempty"`
	Visible     bool     `json:"visible"`
	Locked      bool     `json:"locked"`
}
type Issue struct {
	Level     string `json:"level"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	LayerID   string `json:"layerId"`
	LayerName string `json:"layerName"`
	Field     string `json:"field,omitempty"`
}

func DefaultDocument(w, h int) Document {
	return Document{Width: w, Height: h, Background: "#172f4c", BackgroundMode: "solid", ImageFit: "slice", ImageAnchor: "xMidYMid", Safe: 24, Layers: []Layer{{ID: "title", Kind: "text", Slot: "main_title", Name: "动态主标题", Text: ShortSample, X: float64(w) * .08, Y: float64(h) * .64, W: float64(w) * .84, H: float64(h) * .26, FontSize: math.Round(float64(w) * .075), MinFont: math.Round(float64(w) * .032), FontFamily: "sans-serif", Weight: "700", Color: "#ffffff", Align: "left", LineHeight: 1.15, MaxLines: 2, Wrap: true, AutoShrink: true, Stroke: 3, Visible: true}}}
}

// ValidateDraft allows unfinished content; bounds prevent hostile resource usage.
func ValidateDraft(d Document) error {
	if d.Width < 128 || d.Width > 4096 || d.Height < 128 || d.Height > 4096 || len(d.Layers) > 64 || !finite(d.Safe) || d.Safe < 0 || d.Safe >= float64(min(d.Width, d.Height))/2 {
		return fmt.Errorf("画布尺寸或安全区无效")
	}
	if d.BackgroundMode != "solid" && d.BackgroundMode != "fixed" && d.BackgroundMode != "business" {
		return fmt.Errorf("请选择背景类型")
	}
	if _, e := parseColor(d.Background); e != nil {
		return e
	}
	if d.ImageFit != "" && d.ImageFit != "slice" && d.ImageFit != "meet" {
		return fmt.Errorf("图片填充方式无效")
	}
	switch d.ImageAnchor {
	case "", "xMidYMid", "xMinYMid", "xMaxYMid", "xMidYMin", "xMidYMax":
	default:
		return fmt.Errorf("图片定位无效")
	}
	ids := map[string]bool{}
	for _, l := range d.Layers {
		if l.ID == "" || ids[l.ID] || len(l.ID) > 100 || len([]rune(l.Name)) > 160 || len([]rune(l.Text)) > 2000 {
			return fmt.Errorf("图层标识或文字长度无效")
		}
		ids[l.ID] = true
		if l.Kind != "text" && l.Kind != "decoration" {
			return fmt.Errorf("图层类型无效")
		}
		for _, n := range []float64{l.X, l.Y, l.W, l.H, l.FontSize, l.MinFont, l.Spacing, l.LineHeight, l.Stroke} {
			if !finite(n) || math.Abs(n) > 16384 {
				return fmt.Errorf("图层参数无效")
			}
		}
		if l.W <= 0 || l.H <= 0 || l.Stroke < 0 || l.Stroke > 20 {
			return fmt.Errorf("图层尺寸或描边无效")
		}
		if l.Opacity != nil && (!finite(*l.Opacity) || *l.Opacity < 0 || *l.Opacity > 1) {
			return fmt.Errorf("透明度无效")
		}
		if _, e := parseColor(l.Color); e != nil {
			return e
		}
		if l.Kind == "text" {
			if l.FontSize < 8 || l.FontSize > 600 || l.MinFont < 8 || l.MinFont > l.FontSize || l.MaxLines < 1 || l.MaxLines > 100 || l.MaxChars < 0 || l.MaxChars > 2000 || l.LineHeight < 1 || l.LineHeight > 5 || math.Abs(l.Spacing) > 100 {
				return fmt.Errorf("文字排版参数无效")
			}
			if l.Align != "left" && l.Align != "center" && l.Align != "right" {
				return fmt.Errorf("文字对齐方式无效")
			}
			if l.Weight != "400" && l.Weight != "700" {
				return fmt.Errorf("字重不受支持")
			}
			if l.Slot != "main_title" && l.Slot != "fixed" && l.Slot != "" {
				return fmt.Errorf("文字绑定无效")
			}
			if l.StrokeColor != "" {
				if _, e := parseColor(l.StrokeColor); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func publicationIssues(d Document) []Issue {
	out := []Issue{}
	titles := 0
	for _, l := range d.Layers {
		add := func(level, code, message string) {
			out = append(out, Issue{Level: level, Code: code, Message: message, LayerID: l.ID, LayerName: l.Name})
		}
		if l.Slot == "main_title" {
			titles++
			if !l.Visible {
				add("error", "title", "动态主标题必须可见")
			}
		}
		if !l.Visible {
			continue
		}
		outside := l.X < d.Safe || l.Y < d.Safe || l.X+l.W > float64(d.Width)-d.Safe || l.Y+l.H > float64(d.Height)-d.Safe
		if outside {
			if l.Kind == "decoration" {
				add("warning", "crop", "装饰可能被裁切，请确认效果")
			} else {
				add("error", "safe", "文字超出安全区")
			}
		}
		if l.Kind == "text" && strings.TrimSpace(l.Text) == "" {
			add("error", "empty", "文字内容不能为空")
		}
	}
	if titles != 1 {
		out = append(out, Issue{Level: "error", Code: "title", Message: "必须有且仅有一个动态主标题，隐藏图层仍计入数量", LayerID: "background", LayerName: "画布"})
	}
	return out
}
