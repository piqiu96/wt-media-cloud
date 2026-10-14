package executor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const guideSchema = `{"type":"object","properties":{"guide_video_url":{"type":"string","format":"uri","title":"引导视频下载地址"},"insert_seconds":{"type":"integer","minimum":0,"title":"插入位置（秒）"},"guide_duration_seconds":{"type":"integer","minimum":0,"title":"引导时长（秒，0 为完整视频）"},"max_duration_seconds":{"type":"integer","minimum":0,"title":"成片最长时长（秒，0 为不限）"}},"required":["guide_video_url","insert_seconds","guide_duration_seconds","max_duration_seconds"],"additionalProperties":false}`

type GuideMode string

const (
	GuideInsert GuideMode = "guide_insert"
	GuidePIP    GuideMode = "guide_pip"
)

func GuideInsertV1() Definition {
	return Definition{Code: string(GuideInsert), Version: 1, Schema: json.RawMessage(guideSchema), ExecutorID: "guide_insert_v1", Executor: GuideExecutor{Mode: GuideInsert}}
}

func GuidePIPV1() Definition {
	return Definition{Code: string(GuidePIP), Version: 1, Schema: json.RawMessage(guideSchema), ExecutorID: "guide_pip_v1", Executor: GuideExecutor{Mode: GuidePIP}}
}

func DeployedDefinitions() []Definition {
	return []Definition{BasicCutV1(), GuideInsertV1(), GuidePIPV1()}
}

type GuideExecutor struct{ Mode GuideMode }

type GuidePlan interface {
	Plan
	GuideURL() string
	OutputDurationMS(sourceMS, guideMS int64) (int64, error)
	FFmpegGuideArguments(sourcePath, guidePath, outputPath string, sourceMS, guideMS int64, sourceWidth, sourceHeight int, sourceAudio, guideAudio bool) ([]string, error)
}

type guidePlan struct {
	mode                                    GuideMode
	guideURL                                string
	insertSeconds, guideSeconds, maxSeconds int64
}

func (e GuideExecutor) ValidateParams(raw json.RawMessage) (Plan, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("%s v1 parameters must be an object", e.Mode)
	}
	for name := range fields {
		switch name {
		case "guide_video_url", "insert_seconds", "guide_duration_seconds", "max_duration_seconds":
		default:
			return nil, fmt.Errorf("%s v1 has no parameter %q", e.Mode, name)
		}
	}
	read := func(name string, minimum int64) (int64, error) {
		raw, ok := fields[name]
		if !ok {
			return 0, fmt.Errorf("%s v1 requires %s", e.Mode, name)
		}
		value, err := integerParam(raw, name)
		if err != nil {
			return 0, err
		}
		if value < minimum || value > 86400 {
			return 0, fmt.Errorf("%s is out of range", name)
		}
		return value, nil
	}
	var guideURL string
	if err := json.Unmarshal(fields["guide_video_url"], &guideURL); err != nil {
		return nil, fmt.Errorf("guide_video_url is required")
	}
	if err := ValidateGuideURL(guideURL); err != nil {
		return nil, err
	}
	insert, err := read("insert_seconds", 0)
	if err != nil {
		return nil, err
	}
	guide, err := read("guide_duration_seconds", 0)
	if err != nil {
		return nil, err
	}
	max, err := read("max_duration_seconds", 0)
	if err != nil {
		return nil, err
	}
	return guidePlan{mode: e.Mode, guideURL: guideURL, insertSeconds: insert, guideSeconds: guide, maxSeconds: max}, nil
}

func ValidateGuideURL(value string) error {
	if len(value) == 0 || len(value) > 2048 || strings.TrimSpace(value) != value {
		return fmt.Errorf("guide_video_url must contain 1-2048 characters without surrounding whitespace")
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return fmt.Errorf("guide_video_url must be a public HTTPS address without credentials or a custom port")
	}
	return nil
}

func (p guidePlan) GuideURL() string                     { return p.guideURL }
func (p guidePlan) SegmentMS() (int64, int64)            { return 0, 0 }
func (p guidePlan) FFmpegArguments(_, _ string) []string { return nil }

func (p guidePlan) timing(sourceMS, guideMS int64) (insertMS, effectiveGuideMS, outputMS int64, err error) {
	if sourceMS <= 0 || guideMS <= 0 {
		return 0, 0, 0, fmt.Errorf("source and guide must have positive duration")
	}
	insertMS = min(p.insertSeconds*1000, sourceMS)
	effectiveGuideMS = guideMS
	if p.guideSeconds > 0 {
		effectiveGuideMS = min(effectiveGuideMS, p.guideSeconds*1000)
	}
	outputMS = sourceMS + effectiveGuideMS
	if p.maxSeconds > 0 {
		outputMS = min(outputMS, p.maxSeconds*1000)
	}
	if outputMS < 1000 {
		return 0, 0, 0, fmt.Errorf("output would be shorter than one second")
	}
	return
}

func (p guidePlan) OutputDurationMS(sourceMS, guideMS int64) (int64, error) {
	_, _, output, err := p.timing(sourceMS, guideMS)
	return output, err
}

func (p guidePlan) FFmpegGuideArguments(sourcePath, guidePath, outputPath string, sourceMS, guideMS int64, width, height int, sourceAudio, guideAudio bool) ([]string, error) {
	insertMS, effectiveMS, outputMS, err := p.timing(sourceMS, guideMS)
	if err != nil {
		return nil, err
	}
	if width < 2 || height < 2 {
		return nil, fmt.Errorf("source has no usable video dimensions")
	}
	width -= width % 2
	height -= height % 2
	sec := func(ms int64) string { return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64) }
	insert, duration, sourceEnd := sec(insertMS), sec(effectiveMS), sec(sourceMS)
	canvas := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30,format=yuv420p", width, height, width, height)
	video := make([]string, 0, 8)
	audio := make([]string, 0, 8)
	appendPart := func(input int, start, end string, label string, hasAudio bool, transform string) {
		video = append(video, fmt.Sprintf("[%d:v]trim=start=%s:end=%s,setpts=PTS-STARTPTS,%s[%sv]", input, start, end, transform, label))
		if hasAudio {
			audio = append(audio, fmt.Sprintf("[%d:a]atrim=start=%s:end=%s,asetpts=PTS-STARTPTS,aresample=48000[%sa]", input, start, end, label))
		} else {
			length := sec(effectiveMS)
			if input == 0 {
				startMS, _ := strconv.ParseFloat(start, 64)
				endMS, _ := strconv.ParseFloat(end, 64)
				length = strconv.FormatFloat(endMS-startMS, 'f', 3, 64)
			}
			audio = append(audio, fmt.Sprintf("anullsrc=r=48000:cl=stereo,atrim=duration=%s,asetpts=PTS-STARTPTS[%sa]", length, label))
		}
	}
	labels := []string{}
	if insertMS > 0 {
		appendPart(0, "0", insert, "pre", sourceAudio, canvas)
		labels = append(labels, "pre")
	}
	if p.mode == GuidePIP {
		freezeStart := insertMS
		if freezeStart >= sourceMS {
			freezeStart = max(0, sourceMS-100)
		}
		video = append(video, fmt.Sprintf("[0:v]trim=start=%s:duration=0.100,setpts=PTS-STARTPTS,tpad=stop_mode=clone:stop_duration=%s,trim=duration=%s,%s[background]", sec(freezeStart), duration, duration, canvas))
		video = append(video, fmt.Sprintf("[1:v]trim=duration=%s,setpts=PTS-STARTPTS,scale=%d:%d:force_original_aspect_ratio=decrease,setsar=1,fps=30,format=yuv420p[guide]", duration, width/2, height/2))
		video = append(video, "[background][guide]overlay=x=W-w-20:y=H-h-20:shortest=1[pipv]")
		if guideAudio {
			audio = append(audio, fmt.Sprintf("[1:a]atrim=duration=%s,asetpts=PTS-STARTPTS,aresample=48000[pipa]", duration))
		} else {
			audio = append(audio, fmt.Sprintf("anullsrc=r=48000:cl=stereo,atrim=duration=%s,asetpts=PTS-STARTPTS[pipa]", duration))
		}
		labels = append(labels, "pip")
	} else {
		appendPart(1, "0", duration, "guide", guideAudio, canvas)
		labels = append(labels, "guide")
	}
	if insertMS < sourceMS {
		appendPart(0, insert, sourceEnd, "post", sourceAudio, canvas)
		labels = append(labels, "post")
	}
	graph := append(video, audio...)
	concat := strings.Builder{}
	for _, label := range labels {
		concat.WriteString("[" + label + "v][" + label + "a]")
	}
	graph = append(graph, fmt.Sprintf("%sconcat=n=%d:v=1:a=1[outv][outa]", concat.String(), len(labels)))
	return []string{"-hide_banner", "-loglevel", "error", "-i", sourcePath, "-i", guidePath, "-filter_complex", strings.Join(graph, ";"), "-map", "[outv]", "-map", "[outa]", "-t", sec(outputMS), "-c:v", "libx264", "-c:a", "aac", "-movflags", "+faststart", "-y", outputPath}, nil
}
