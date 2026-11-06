package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func seconds(value float64) string { return strconv.FormatFloat(value, 'f', 6, 64) }
func (p dynamicPlan) Compile(input RenderInput) (RenderResult, error) {
	var result RenderResult
	cfg, err := parseDynamicDefaults(input.Defaults)
	if err != nil {
		return result, err
	}
	intro, ok := input.Resources["intro"]
	if !ok {
		return result, errors.New("intro resource is required")
	}
	ad, ok := input.Resources["ad"]
	if !ok {
		return result, errors.New("ad resource is required")
	}
	ending, ok := input.Resources["ending"]
	if !ok {
		return result, errors.New("ending resource is required")
	}
	if !intro.HasAudio || !ad.HasAudio || !ending.HasAudio {
		return result, errors.New("intro, ad and ending must carry audio")
	}
	if math.Abs(ending.Duration-cfg.TailSeconds) > .15 {
		return result, errors.New("ending duration does not match the configured slice library")
	}
	timeline, err := p.Timeline(input.Source.Duration, intro.Duration, ad.Duration, ending.Duration, input.Seed)
	if err != nil {
		return result, err
	}
	for _, media := range []MediaInput{input.Source, intro, ad, ending} {
		if media.Path == "" || media.Width < 2 || media.Height < 2 {
			return result, errors.New("resource has no usable video")
		}
	}
	graph := []string{"[0:v]split=4[source_intro][source_pre][source_ad][source_post]"}
	if input.Source.HasAudio {
		graph = append(graph, "[0:a]asplit=2[source_pre_audio][source_post_audio]")
	}
	normalize := fmt.Sprintf("setsar=1,fps=%d,settb=AVTB,format=yuv420p", cfg.FPS)
	background := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,%s,boxblur=%d:1,eq=brightness=%s", cfg.Width, cfg.Height, cfg.Width, cfg.Height, normalize, cfg.Blur, seconds(cfg.Brightness))
	fit := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2,%s", cfg.Width, cfg.Height, normalize)
	audio := func(stream, label string, start, duration float64, fade bool) {
		chain := fmt.Sprintf("atrim=start=%s:duration=%s,asetpts=PTS-STARTPTS,aresample=48000,aformat=sample_fmts=fltp:channel_layouts=stereo,apad,atrim=duration=%s", seconds(start), seconds(duration), seconds(duration))
		if fade {
			chain += fmt.Sprintf(",afade=t=out:st=%s:d=%s", seconds(duration-cfg.TailFade), seconds(cfg.TailFade))
		}
		graph = append(graph, fmt.Sprintf("[%s]%s[%sa]", stream, chain, label))
	}
	silent := func(label string, duration float64) {
		graph = append(graph, fmt.Sprintf("anullsrc=r=48000:cl=stereo,atrim=duration=%s,asetpts=PTS-STARTPTS[%sa]", seconds(duration), label))
	}
	pip := func(source, foreground, label string, start, duration, ratio float64) {
		graph = append(graph, fmt.Sprintf("[%s]trim=start=%s:duration=%s,setpts=PTS-STARTPTS,%s[%sbg]", source, seconds(start), seconds(duration), background, label))
		width := int(float64(cfg.Width)*ratio) / 2 * 2
		height := cfg.Height - 4*cfg.ShadowBlur - 2*cfg.ShadowOffset
		graph = append(graph, fmt.Sprintf("[%s]trim=duration=%s,setpts=PTS-STARTPTS,scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1,fps=%d,settb=AVTB,format=rgba,split=2[%sfg][%sshadow0]", foreground, seconds(duration), width, height, cfg.FPS, label, label))
		pad := cfg.ShadowBlur * 2
		graph = append(graph, fmt.Sprintf("[%sshadow0]colorchannelmixer=rr=0:gg=0:bb=0:aa=%s,pad=iw+%d:ih+%d:%d:%d:color=black@0,gblur=sigma=%d[%sshadow]", label, seconds(cfg.ShadowOpacity), pad*2, pad*2, pad, pad, cfg.ShadowBlur, label))
		graph = append(graph, fmt.Sprintf("[%sbg][%sshadow]overlay=x=(W-w)/2:y=(H-h)/2+%d:shortest=1[%sshaded]", label, label, cfg.ShadowOffset, label))
		graph = append(graph, fmt.Sprintf("[%sshaded][%sfg]overlay=x=(W-w)/2:y=(H-h)/2:shortest=1,format=yuv420p[%sv]", label, label, label))
	}
	normal := func(stream, label string, start, duration float64, fade bool) {
		graph = append(graph, fmt.Sprintf("[%s]trim=start=%s:duration=%s,setpts=PTS-STARTPTS,split=2[%sbg0][%sfg0]", stream, seconds(start), seconds(duration), label, label))
		graph = append(graph, fmt.Sprintf("[%sbg0]%s[%sbg]", label, background, label))
		graph = append(graph, fmt.Sprintf("[%sfg0]%s[%sfg]", label, fit, label))
		chain := fmt.Sprintf("[%sbg][%sfg]overlay=x=(W-w)/2:y=(H-h)/2:shortest=1,format=yuv420p", label, label)
		if fade {
			chain += fmt.Sprintf(",fade=t=out:st=%s:d=%s", seconds(duration-cfg.TailFade), seconds(cfg.TailFade))
		}
		graph = append(graph, chain+"["+label+"v]")
	}
	pre, post := timeline.Segments[1], timeline.Segments[3]
	pip("source_intro", "1:v", "intro", p.Start, intro.Duration, p.IntroWidth)
	normal("source_pre", "pre", p.Start, pre.Duration, false)
	pip("source_ad", "2:v", "ad", p.Start+timeline.Insert, ad.Duration, p.AdWidth)
	normal("source_post", "post", p.Start+timeline.Insert, post.Duration, false)
	normal("3:v", "tail", 0, ending.Duration, true)
	audio("1:a", "intro", 0, intro.Duration, false)
	audio("2:a", "ad", 0, ad.Duration, false)
	audio("3:a", "tail", 0, ending.Duration, true)
	if input.Source.HasAudio {
		audio("source_pre_audio", "pre", p.Start, pre.Duration, false)
		audio("source_post_audio", "post", p.Start+timeline.Insert, post.Duration, false)
	} else {
		silent("pre", pre.Duration)
		silent("post", post.Duration)
	}
	labels := []string{"intro", "pre", "ad", "post", "tail"}
	videoLabel, audioLabel := "introv", "introa"
	if p.Transition == 0 {
		pairs := ""
		for _, label := range labels {
			pairs += "[" + label + "v][" + label + "a]"
		}
		graph = append(graph, pairs+"concat=n=5:v=1:a=1[outv][outa]")
		videoLabel, audioLabel = "outv", "outa"
	} else {
		for i := 1; i < len(labels); i++ {
			nextV, nextA := fmt.Sprintf("xfv%d", i), fmt.Sprintf("xfa%d", i)
			graph = append(graph, fmt.Sprintf("[%s][%sv]xfade=transition=fade:duration=%s:offset=%s[%s]", videoLabel, labels[i], seconds(p.Transition), seconds(timeline.Segments[i].Start), nextV))
			graph = append(graph, fmt.Sprintf("[%s][%sa]acrossfade=d=%s:c1=tri:c2=tri[%s]", audioLabel, labels[i], seconds(p.Transition), nextA))
			videoLabel, audioLabel = nextV, nextA
		}
	}
	result.Arguments = []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-protocol_whitelist", "file,pipe", "-i", input.Source.Path, "-i", intro.Path, "-i", ad.Path, "-i", ending.Path, "-filter_complex_threads", strconv.Itoa(cfg.FilterThreads), "-filter_complex", strings.Join(graph, ";"), "-map", "[" + videoLabel + "]", "-map", "[" + audioLabel + "]", "-t", seconds(timeline.Duration), "-c:v", "libx264", "-crf", strconv.Itoa(cfg.CRF), "-preset", cfg.Preset, "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart", "-map_metadata", "-1", "-y", input.OutputPath}
	result.DurationMS = int64(math.Round(timeline.Duration * 1000))
	result.Timeline, _ = json.Marshal(timeline)
	return result, nil
}
