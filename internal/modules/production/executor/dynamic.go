package executor

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"strings"
)

const dynamicSchema = `{"type":"object","properties":{
 "intro_video_url":{"type":"string","format":"uri","title":"开头引导视频URL","editable":false},
 "ad_video_url":{"type":"string","format":"uri","title":"广告引导视频URL","editable":false},
 "start_seconds":{"type":"number","minimum":0,"title":"正片开始秒数","editable":true},
 "end_seconds":{"type":"number","minimum":0,"title":"正片结束秒数（0为自动）","auto_zero":true,"editable":true},
 "max_duration_seconds":{"type":"number","minimum":1,"title":"成片时长上限（秒）","maximum_from_strategy":true,"editable":true},
 "ad_start_min_seconds":{"type":"number","minimum":0,"title":"广告出现最早秒数","editable":false},
 "ad_start_max_seconds":{"type":"number","minimum":0,"title":"广告出现最晚秒数","editable":false},
 "intro_pip_width":{"type":"number","minimum":0.1,"maximum":1,"title":"开头画中画宽度比例","editable":false},
 "ad_pip_width":{"type":"number","minimum":0.1,"maximum":1,"title":"广告画中画宽度比例","editable":false},
 "transition_seconds":{"type":"number","minimum":0,"maximum":1,"title":"段落转场时长（秒）","editable":false}
 },"required":["intro_video_url","ad_video_url","start_seconds","end_seconds","max_duration_seconds","ad_start_min_seconds","ad_start_max_seconds","intro_pip_width","ad_pip_width","transition_seconds"],"additionalProperties":false}`

type DynamicExecutor struct{ omitIntro bool }

func DynamicPIPEndingV1() Definition {
	return Definition{Code: "dynamic_pip_ending", Version: 1, Schema: json.RawMessage(dynamicSchema), ExecutorID: "dynamic_pip_ending_v1", Executor: DynamicExecutor{}}
}

func AdPIPEndingV1() Definition {
	var schema map[string]any
	_ = json.Unmarshal([]byte(dynamicSchema), &schema)
	properties := schema["properties"].(map[string]any)
	delete(properties, "intro_video_url")
	delete(properties, "intro_pip_width")
	schema["description"] = "正片前段 → 广告画中画 → 从插入点继续正片 → 配乐片尾。广告以插入点起等长正片作模糊背景，仅播放广告声音；广告无音轨时该段静音。"
	properties["ad_video_url"].(map[string]any)["title"] = "广告视频URL"
	descriptions := map[string]string{
		"ad_video_url":         "广告以前景画中画播放，广告无音轨时该段静音。",
		"start_seconds":        "正片选段起点。广告结束后从其插入点继续正片。",
		"end_seconds":          "正片选段终点，0表示自动选取。",
		"max_duration_seconds": "包含广告、片尾及转场重叠的成片时长上限。",
		"ad_start_min_seconds": "广告在成片中出现的最早时间。",
		"ad_start_max_seconds": "广告在成片中出现的最晚时间。",
		"ad_pip_width":         "广告前景相对于成片宽度的比例。",
		"transition_seconds":   "相邻段落之间的淡入淡出重叠时长。",
	}
	for key, description := range descriptions {
		properties[key].(map[string]any)["description"] = description
	}
	required := []string{}
	for _, value := range schema["required"].([]any) {
		key := value.(string)
		if key != "intro_video_url" && key != "intro_pip_width" {
			required = append(required, key)
		}
	}
	schema["required"] = required
	raw, _ := json.Marshal(schema)
	return Definition{Code: "ad_pip_ending", Version: 1, Schema: raw, ExecutorID: "ad_pip_ending_v1", Executor: DynamicExecutor{omitIntro: true}}
}

type DynamicParams struct {
	IntroURL    string  `json:"intro_video_url"`
	AdURL       string  `json:"ad_video_url"`
	Start       float64 `json:"start_seconds"`
	End         float64 `json:"end_seconds"`
	MaxDuration float64 `json:"max_duration_seconds"`
	AdMin       float64 `json:"ad_start_min_seconds"`
	AdMax       float64 `json:"ad_start_max_seconds"`
	IntroWidth  float64 `json:"intro_pip_width"`
	AdWidth     float64 `json:"ad_pip_width"`
	Transition  float64 `json:"transition_seconds"`
}
type DynamicPlan interface {
	Plan
	Timeline(source, intro, ad, tail float64, seed int64) (DynamicTimeline, error)
}
type dynamicPlan struct {
	DynamicParams
	omitIntro bool
}
type Segment struct {
	Name        string  `json:"name"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Duration    float64 `json:"duration"`
	SourceStart float64 `json:"source_start"`
	SourceEnd   float64 `json:"source_end"`
}
type DynamicTimeline struct {
	Duration    float64   `json:"duration"`
	AdStart     float64   `json:"ad_start"`
	Insert      float64   `json:"source_insert"`
	MainStart   float64   `json:"main_start"`
	MainEnd     float64   `json:"main_end"`
	Transitions int       `json:"transitions"`
	Overlap     float64   `json:"overlap"`
	Segments    []Segment `json:"segments"`
}
type DynamicDefaults struct {
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FPS            int     `json:"fps"`
	CRF            int     `json:"crf"`
	Preset         string  `json:"preset"`
	Blur           int     `json:"blur"`
	Brightness     float64 `json:"brightness"`
	ShadowBlur     int     `json:"shadow_blur"`
	ShadowOpacity  float64 `json:"shadow_opacity"`
	ShadowOffset   int     `json:"shadow_offset"`
	TailSeconds    float64 `json:"tail_seconds"`
	TailFade       float64 `json:"tail_fade"`
	EndingIndexKey string  `json:"ending_index_key"`
	FilterThreads  int     `json:"filter_threads"`
}

func strictJSON(raw json.RawMessage, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if d.More() {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}
func (e DynamicExecutor) ValidateParams(raw json.RawMessage) (Plan, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, errors.New("dynamic template parameters must be an object")
	}
	var schema struct {
		Required []string `json:"required"`
	}
	schemaRaw := json.RawMessage(dynamicSchema)
	if e.omitIntro {
		schemaRaw = AdPIPEndingV1().Schema
	}
	_ = json.Unmarshal(schemaRaw, &schema)
	for _, key := range schema.Required {
		if len(fields[key]) == 0 || bytes.Equal(fields[key], []byte("null")) {
			return nil, fmt.Errorf("missing parameter %s", key)
		}
	}
	if e.omitIntro {
		if _, exists := fields["intro_video_url"]; exists {
			return nil, errors.New("intro is not supported by ad_pip_ending")
		}
		if _, exists := fields["intro_pip_width"]; exists {
			return nil, errors.New("intro width is not supported by ad_pip_ending")
		}
	}
	var p DynamicParams
	if err := strictJSON(raw, &p); err != nil {
		return nil, err
	}
	for _, v := range []float64{p.Start, p.End, p.MaxDuration, p.AdMin, p.AdMax, p.IntroWidth, p.AdWidth, p.Transition} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("non-finite template parameter")
		}
	}
	if p.Start < 0 || p.End < 0 || (p.End > 0 && p.End <= p.Start) || p.MaxDuration < 1 || p.MaxDuration > 86400 || p.AdMin < 0 || p.AdMax < p.AdMin || p.AdMax >= p.MaxDuration || (!e.omitIntro && (p.IntroWidth < .1 || p.IntroWidth > 1)) || p.AdWidth < .1 || p.AdWidth > 1 || p.Transition < 0 || p.Transition > 1 {
		return nil, errors.New("invalid range in dynamic template parameters")
	}
	if !e.omitIntro {
		if err := ValidateMediaURL(p.IntroURL); err != nil {
			return nil, err
		}
	}
	if err := ValidateMediaURL(p.AdURL); err != nil {
		return nil, err
	}
	return dynamicPlan{DynamicParams: p, omitIntro: e.omitIntro}, nil
}
func FileResourceToken(value string) (string, bool) {
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	token := strings.TrimPrefix(u.Path, "/api/v1/file-resources/")
	if token == u.Path || len(token) != 64 {
		return "", false
	}
	if _, err = hex.DecodeString(token); err != nil {
		return "", false
	}
	return token, true
}
func ValidateMediaURL(value string) error {
	if len(value) > 2048 || strings.TrimSpace(value) != value {
		return errors.New("invalid media URL")
	}
	if _, ok := FileResourceToken(value); ok {
		return nil
	}
	return ValidatePublicMediaURL(value)
}
func parseDynamicDefaults(raw json.RawMessage) (DynamicDefaults, error) {
	var c DynamicDefaults
	if err := strictJSON(raw, &c); err != nil {
		return c, err
	}
	if c.Width < 2 || c.Height < 2 || c.Width%2 != 0 || c.Height%2 != 0 || c.FPS < 1 || c.FPS > 120 || c.CRF < 0 || c.CRF > 51 || c.Blur < 1 || c.ShadowBlur < 1 || c.ShadowOpacity < 0 || c.ShadowOpacity > 1 || c.ShadowOffset < 0 || c.Brightness > 0 || c.Brightness < -1 || c.TailSeconds <= 0 || c.TailFade <= 0 || c.TailFade >= c.TailSeconds || c.FilterThreads < 1 || c.EndingIndexKey == "" || strings.Contains(c.EndingIndexKey, "..") {
		return c, errors.New("invalid dynamic template render defaults")
	}
	if c.Height-4*c.ShadowBlur-2*c.ShadowOffset < 2 {
		return c, errors.New("shadow leaves no foreground height")
	}
	switch c.Preset {
	case "ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow":
	default:
		return c, errors.New("invalid H264 preset")
	}
	return c, nil
}
func (DynamicExecutor) ValidateDefaults(raw json.RawMessage) error {
	_, err := parseDynamicDefaults(raw)
	return err
}
func (p dynamicPlan) Resources(raw json.RawMessage) ([]ResourceRequest, error) {
	c, err := parseDynamicDefaults(raw)
	if err != nil {
		return nil, err
	}
	resources := []ResourceRequest{{Name: "ad", URL: p.AdURL}, {Name: "ending", PoolIndexKey: c.EndingIndexKey}}
	if !p.omitIntro {
		resources = append([]ResourceRequest{{Name: "intro", URL: p.IntroURL}}, resources...)
	}
	return resources, nil
}
func (p dynamicPlan) Timeline(source, intro, ad, tail float64, seed int64) (DynamicTimeline, error) {
	names := []string{"opening_pip", "source_before_ad", "ad_pip", "source_after_ad", "ending"}
	if p.omitIntro {
		names = names[1:]
		intro = 0
	}
	t := DynamicTimeline{MainStart: p.Start, Overlap: p.Transition, Transitions: len(names) - 1}
	durations := []float64{source, ad, tail}
	if !p.omitIntro {
		durations = append(durations, intro)
	}
	for _, d := range durations {
		if d <= 0 || math.IsNaN(d) || math.IsInf(d, 0) {
			return t, errors.New("all video resources must have positive duration")
		}
	}
	availableEnd := source
	if p.End > 0 {
		if p.End > source+.001 {
			return t, errors.New("selected source end exceeds video")
		}
		availableEnd = p.End
	}
	available := availableEnd - p.Start
	budget := p.MaxDuration - intro - ad - tail + float64(t.Transitions)*p.Transition
	main := math.Min(available, budget)
	t.AdStart = p.AdMin + rand.New(rand.NewSource(seed)).Float64()*(p.AdMax-p.AdMin)
	t.Insert = t.AdStart - intro + 2*p.Transition
	if p.omitIntro {
		t.Insert = t.AdStart + p.Transition
	}
	preOverlap := 2 * p.Transition
	if p.omitIntro {
		preOverlap = p.Transition
	}
	if main <= 0 || t.Insert <= preOverlap || main-t.Insert <= 2*p.Transition {
		return t, errors.New("intro/ad placement leaves an invalid source segment")
	}
	if intro > available+.001 || t.Insert+ad > available+.001 {
		return t, errors.New("source cannot supply an equal-length dynamic background")
	}
	lengths := []float64{intro, t.Insert, ad, main - t.Insert, tail}
	sourceStarts := []float64{p.Start, p.Start, p.Start + t.Insert, p.Start + t.Insert, 0}
	if p.omitIntro {
		lengths = lengths[1:]
		sourceStarts = sourceStarts[1:]
	}
	cursor := 0.0
	for i, d := range lengths {
		connections := 2
		if i == 0 || i == len(lengths)-1 {
			connections = 1
		}
		if d <= float64(connections)*p.Transition {
			return t, errors.New("segment is too short for transitions")
		}
		start := cursor
		if i > 0 {
			start -= p.Transition
		}
		cursor = start + d
		t.Segments = append(t.Segments, Segment{Name: names[i], Start: start, End: cursor, Duration: d, SourceStart: sourceStarts[i], SourceEnd: sourceStarts[i] + d})
	}
	t.Duration = cursor
	t.MainEnd = p.Start + main
	if t.Duration > p.MaxDuration+.001 {
		return t, errors.New("timeline exceeds output limit")
	}
	return t, nil
}
