package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/png"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

func parseColor(s string) (color.NRGBA, error) {
	if !strings.HasPrefix(s, "#") {
		return color.NRGBA{}, fmt.Errorf("颜色无效")
	}
	raw := s[1:]
	if len(raw) == 3 || len(raw) == 4 {
		var b strings.Builder
		for _, c := range raw {
			b.WriteRune(c)
			b.WriteRune(c)
		}
		raw = b.String()
	}
	if len(raw) == 6 {
		raw += "ff"
	}
	if len(raw) != 8 {
		return color.NRGBA{}, fmt.Errorf("颜色无效")
	}
	v, e := strconv.ParseUint(raw, 16, 32)
	if e != nil {
		return color.NRGBA{}, fmt.Errorf("颜色无效")
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}

var errTextOverflow = errors.New("文字区域不足，请缩短标题或调整文字区域")

func Render(ctx context.Context, d Document, title string, background image.Image) ([]byte, []Issue, error) {
	if e := ValidateDraft(d); e != nil {
		return nil, nil, e
	}
	// Copy before substituting input: published documents remain immutable.
	d.Layers = append([]Layer(nil), d.Layers...)
	for i := range d.Layers {
		if d.Layers[i].Slot == "main_title" {
			d.Layers[i].Text = title
		}
	}
	issues := publicationIssues(d)
	add := func(l Layer, code, field, message string) {
		issues = append(issues, Issue{Level: "error", Code: code, Field: field, Message: message, LayerID: l.ID, LayerName: l.Name})
	}
	out := image.NewRGBA(image.Rect(0, 0, d.Width, d.Height))
	bg, _ := parseColor(d.Background)
	stddraw.Draw(out, out.Bounds(), image.NewUniform(bg), image.Point{}, stddraw.Src)
	if d.BackgroundMode != "solid" {
		if background == nil {
			add(Layer{ID: "background", Name: "背景"}, "background", "image", "缺少可读取的底图")
		} else {
			fitBackground(out, background, d)
		}
	}
	for _, l := range d.Layers {
		if e := ctx.Err(); e != nil {
			return nil, issues, e
		}
		if !l.Visible {
			continue
		}
		c, _ := parseColor(l.Color)
		if l.Kind == "decoration" {
			if l.Opacity != nil {
				c.A = uint8(float64(c.A) * *l.Opacity)
			}
			r := image.Rect(int(math.Round(l.X)), int(math.Round(l.Y)), int(math.Round(l.X+l.W)), int(math.Round(l.Y+l.H)))
			stddraw.Draw(out, r, image.NewUniform(c), image.Point{}, stddraw.Over)
			continue
		}
		count := utf8.RuneCountInString(l.Text)
		if l.MaxChars > 0 && count > l.MaxChars {
			add(l, "max_chars", "maxChars", fmt.Sprintf("共%d字，超过上限%d字，请缩短文字或调整最多字数", count, l.MaxChars))
			continue
		}
		if e := drawText(out, l); e != nil {
			code, field := "font", "fontFamily"
			if errors.Is(e, errTextOverflow) {
				code, field = "layout", "w"
			}
			add(l, code, field, e.Error())
		}
	}
	for _, issue := range issues {
		if issue.Level == "error" {
			return nil, issues, fmt.Errorf("封面校验未通过")
		}
	}
	var data bytes.Buffer
	if e := png.Encode(&data, out); e != nil {
		return nil, issues, e
	}
	return data.Bytes(), issues, nil
}
func fitBackground(out *image.RGBA, src image.Image, d Document) {
	b := src.Bounds()
	position := func(space int, axis string) int {
		if strings.Contains(d.ImageAnchor, axis+"Min") {
			return 0
		}
		if strings.Contains(d.ImageAnchor, axis+"Max") {
			return space
		}
		return space / 2
	}
	if d.ImageFit == "meet" {
		scale := math.Min(float64(d.Width)/float64(b.Dx()), float64(d.Height)/float64(b.Dy()))
		w, h := max(1, int(math.Round(float64(b.Dx())*scale))), max(1, int(math.Round(float64(b.Dy())*scale)))
		x, y := position(d.Width-w, "x"), position(d.Height-h, "Y")
		draw.CatmullRom.Scale(out, image.Rect(x, y, x+w, y+h), src, b, stddraw.Over, nil)
		return
	}
	// Crop the source before scaling. A very wide image never creates an enormous
	// intermediate destination buffer merely to crop it back to the cover canvas.
	w, h := b.Dx(), b.Dy()
	ratio := float64(d.Width) / float64(d.Height)
	if float64(w)/float64(h) > ratio {
		w = max(1, int(math.Round(float64(h)*ratio)))
	} else {
		h = max(1, int(math.Round(float64(w)/ratio)))
	}
	x, y := b.Min.X+position(b.Dx()-w, "x"), b.Min.Y+position(b.Dy()-h, "Y")
	draw.CatmullRom.Scale(out, out.Bounds(), src, image.Rect(x, y, x+w, y+h), stddraw.Over, nil)
}

type textLayout struct {
	lines []string
	size  float64
	face  font.Face
}

func layout(l Layer) (textLayout, error) {
	f, e := loadFont(l.FontFamily, l.Weight)
	if e != nil {
		return textLayout{}, e
	}
	for _, r := range l.Text {
		if r == '\n' {
			continue
		}
		index, e := f.font.GlyphIndex(nil, r)
		if e != nil || index == 0 {
			return textLayout{}, fmt.Errorf("字体缺少字符：%c", r)
		}
	}
	size := math.Round(l.FontSize)
	minimum := math.Round(l.MinFont)
	for ; size >= minimum; size-- {
		face, e := opentype.NewFace(f.font, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
		if e != nil {
			return textLayout{}, e
		}
		width := func(s string) float64 {
			return float64(font.MeasureString(face, s))/64 + float64(max(0, utf8.RuneCountInString(s)-1))*l.Spacing
		}
		lines := []string{}
		for _, paragraph := range strings.Split(l.Text, "\n") {
			line := ""
			for _, ch := range paragraph {
				if line != "" && l.Wrap && width(line+string(ch)) > l.W {
					lines = append(lines, line)
					line = ""
				}
				line += string(ch)
			}
			lines = append(lines, line)
		}
		overflow := len(lines) > l.MaxLines || float64(len(lines))*size*l.LineHeight > l.H
		for _, line := range lines {
			overflow = overflow || width(line) > l.W
		}
		if !overflow {
			return textLayout{lines: lines, size: size, face: face}, nil
		}
		face.Close()
		if !l.AutoShrink || size == minimum {
			break
		}
	}
	return textLayout{}, errTextOverflow
}
func drawText(out *image.RGBA, l Layer) error {
	result, e := layout(l)
	if e != nil {
		return e
	}
	defer result.face.Close()
	c, _ := parseColor(l.Color)
	stroke := color.NRGBA{A: 255}
	if l.StrokeColor != "" {
		stroke, _ = parseColor(l.StrokeColor)
	}
	for i, line := range result.lines {
		width := float64(font.MeasureString(result.face, line))/64 + float64(max(0, utf8.RuneCountInString(line)-1))*l.Spacing
		x := l.X
		if l.Align == "center" {
			x += (l.W - width) / 2
		}
		if l.Align == "right" {
			x += l.W - width
		}
		y := l.Y + result.size + float64(i)*result.size*l.LineHeight
		if l.Shadow {
			drawShadow(out, result.face, line, x+3, y+3, l.Spacing, width, result.size)
		}
		// Circular stroke uses a bounded perimeter, rather than changing glyph width.
		if l.Stroke > 0 {
			steps := max(8, int(math.Ceil(2*math.Pi*l.Stroke)))
			for j := 0; j < steps; j++ {
				angle := 2 * math.Pi * float64(j) / float64(steps)
				paintLine(out, result.face, line, x+math.Cos(angle)*l.Stroke/2, y+math.Sin(angle)*l.Stroke/2, l.Spacing, stroke)
			}
		}
		paintLine(out, result.face, line, x, y, l.Spacing, c)
	}
	return nil
}
func paintLine(out stddraw.Image, face font.Face, line string, x, y, spacing float64, c color.NRGBA) {
	d := font.Drawer{Dst: out, Src: image.NewUniform(c), Face: face, Dot: fixed.Point26_6{X: fixed.Int26_6(math.Round(x * 64)), Y: fixed.Int26_6(math.Round(y * 64))}}
	if spacing == 0 {
		d.DrawString(line)
		return
	}
	var prev rune
	for i, r := range line {
		if i > 0 {
			d.Dot.X += face.Kern(prev, r) + fixed.Int26_6(math.Round(spacing*64))
		}
		d.DrawString(string(r))
		prev = r
	}
}

// The editor uses a 4px Gaussian drop shadow. Render only its bounded text region.
func drawShadow(out *image.RGBA, face font.Face, line string, x, y, spacing, width, size float64) {
	const radius = 12
	bounds := image.Rect(int(math.Floor(x))-radius, int(math.Floor(y-size))-radius, int(math.Ceil(x+width))+radius, int(math.Ceil(y+size/2))+radius).Intersect(out.Bounds())
	if bounds.Empty() {
		return
	}
	mask := image.NewAlpha(bounds)
	paintLine(mask, face, line, x, y, spacing, color.NRGBA{A: 153})
	kernel := make([]float64, radius*2+1)
	total := 0.0
	for i := -radius; i <= radius; i++ {
		v := math.Exp(-float64(i*i) / 32)
		kernel[i+radius] = v
		total += v
	}
	for i := range kernel {
		kernel[i] /= total
	}
	w, h := bounds.Dx(), bounds.Dy()
	horizontal := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for k := -radius; k <= radius; k++ {
				if x+k >= 0 && x+k < w {
					horizontal[y*w+x] += float64(mask.Pix[y*mask.Stride+x+k]) * kernel[k+radius]
				}
			}
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			value := 0.0
			for k := -radius; k <= radius; k++ {
				if y+k >= 0 && y+k < h {
					value += horizontal[(y+k)*w+x] * kernel[k+radius]
				}
			}
			mask.Pix[y*mask.Stride+x] = uint8(math.Round(value))
		}
	}
	stddraw.DrawMask(out, bounds, image.NewUniform(color.Black), image.Point{}, mask, bounds.Min, stddraw.Over)
}
