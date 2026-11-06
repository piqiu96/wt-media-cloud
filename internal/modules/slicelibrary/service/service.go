package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/sourcefile"
	"github.com/wt-media/wt-media-cloud/internal/infra/media"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/repository"
)

type Candidate struct {
	SourceID        string        `json:"source_id"`
	Title           string        `json:"title"`
	SHA256          string        `json:"sha256"`
	Size            int64         `json:"size"`
	Info            media.Info    `json:"media"`
	Profile         model.Profile `json:"profile"`
	ReviewedSeconds float64       `json:"reviewed_seconds"`
}
type Decision struct {
	SourceID        string  `json:"source_id"`
	SHA256          string  `json:"sha256"`
	Suitable        bool    `json:"suitable"`
	Title           string  `json:"short_title"`
	Reason          string  `json:"reason"`
	ReviewedSeconds float64 `json:"reviewed_seconds"`
}
type Service struct {
	Config     config.SliceLibraryConfig
	Tools      config.FFmpegConfig
	Store      storage.ImmutableStore
	Repository repository.Repository
}

func ShortTitle(title string) string {
	var result []rune
	for _, r := range strings.TrimSpace(title) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' {
			result = append(result, r)
		} else if unicode.IsSpace(r) && len(result) > 0 && result[len(result)-1] != '_' {
			result = append(result, '_')
		}
		if len(result) >= 32 {
			break
		}
	}
	out := strings.Trim(string(result), "_-")
	if out == "" {
		return "舞蹈视频"
	}
	return out
}
func ClipCount(duration, seconds float64, cap int) int {
	if seconds <= 0 || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0
	}
	return min(int(math.Floor(duration/seconds)), cap)
}

func UsableDuration(info media.Info) float64 {
	duration := info.Duration
	for _, stream := range []float64{info.VideoDuration, info.AudioDuration} {
		if stream > 0 {
			duration = math.Min(duration, stream)
		}
	}
	return duration
}
func ValidateDecision(c Candidate, d Decision) error {
	if d.SourceID != c.SourceID || d.SHA256 != c.SHA256 {
		return errors.New("review does not match downloaded source")
	}
	if math.IsNaN(d.ReviewedSeconds) || d.ReviewedSeconds != c.ReviewedSeconds {
		return errors.New("review does not match sampled range")
	}
	if strings.TrimSpace(d.Reason) == "" {
		return errors.New("review reason is required")
	}
	if d.Suitable && strings.TrimSpace(d.Title) == "" {
		return errors.New("approved source needs a short title")
	}
	return nil
}

func ApprovedProfile(c Candidate, requestedCap int) (model.Profile, error) {
	p := c.Profile
	if p.ClipSeconds <= 0 || p.MaxClips < 1 || p.CRF < 0 || p.CRF > 51 || p.Preset == "" || math.IsNaN(c.ReviewedSeconds) || math.IsInf(c.ReviewedSeconds, 0) || float64(p.MaxClips)*p.ClipSeconds > c.ReviewedSeconds+0.001 {
		return p, errors.New("candidate profile exceeds reviewed range")
	}
	p.MaxClips = min(p.MaxClips, requestedCap)
	if p.MaxClips < 1 {
		return p, errors.New("clip budget is empty")
	}
	return p, nil
}
func (s Service) Validate() error {
	c := s.Config
	if c.ClipSeconds <= 0 || c.MaxCandidates < 1 || c.MaxClipsPerSource < 1 || c.MaxSourceBytes < 1 || c.Timeout.Duration <= 0 || c.SampleCount < 2 || c.SampleCount > 32 || c.CRF < 0 || c.CRF > 51 || c.Preset == "" {
		return errors.New("invalid slice library config")
	}
	if c.Prefix == "" || strings.HasPrefix(c.Prefix, "/") || strings.Contains(c.Prefix, "..") {
		return errors.New("invalid slice prefix")
	}
	return nil
}
func HashFile(filename string) (string, int64, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

// Prepare creates only local artifacts. A technical failure never reaches the
// model or the published library. Signed CDN URLs are not persisted.
func (s Service) Prepare(ctx context.Context, root, id, title, address string) (directory string, err error) {
	if err = s.Validate(); err != nil {
		return "", err
	}
	if id == "" || len(id) > 32 {
		return "", errors.New("invalid Douyin video ID")
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return "", errors.New("invalid Douyin video ID")
		}
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	directory, err = os.MkdirTemp(root, "candidate-"+id+"-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(directory)
		}
	}()
	body, info, err := sourcefile.Open(ctx, address, 0)
	if err != nil {
		return directory, errors.New("candidate download failed")
	}
	defer body.Close()
	if info.TotalBytes > s.Config.MaxSourceBytes {
		return directory, errors.New("candidate exceeds size budget")
	}
	filename := filepath.Join(directory, "source.mp4")
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return directory, err
	}
	n, copyErr := io.Copy(f, io.LimitReader(body, s.Config.MaxSourceBytes+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || n > s.Config.MaxSourceBytes {
		return directory, errors.New("incomplete or oversized download")
	}
	mediaInfo, err := media.ProbeFile(ctx, s.Tools.FFprobePath, filename)
	if err != nil {
		return directory, err
	}
	mediaInfo.Duration = UsableDuration(mediaInfo)
	if !mediaInfo.HasAudio || ClipCount(mediaInfo.Duration, s.Config.ClipSeconds, s.Config.MaxClipsPerSource) == 0 {
		return directory, errors.New("candidate needs audio and a complete clip")
	}
	if err = s.ffmpeg(ctx, "-i", filename, "-f", "null", "-"); err != nil {
		return directory, errors.New("candidate cannot fully decode")
	}
	// Sample across the actual sliceable span, so the review covers every clip.
	span := float64(ClipCount(mediaInfo.Duration, s.Config.ClipSeconds, s.Config.MaxClipsPerSource)) * s.Config.ClipSeconds
	interval := span / float64(s.Config.SampleCount)
	filter := fmt.Sprintf("fps=1/%g,scale=320:-2,tile=4x%d", interval, (s.Config.SampleCount+3)/4)
	if err = s.ffmpeg(ctx, "-t", fmt.Sprint(span), "-i", filename, "-vf", filter, "-frames:v", "1", filepath.Join(directory, "frames.jpg")); err != nil {
		return directory, err
	}
	if err = s.ffmpeg(ctx, "-t", fmt.Sprint(span), "-i", filename, "-vn", "-ac", "1", "-ar", "16000", filepath.Join(directory, "audio.wav")); err != nil {
		return directory, err
	}
	digest, size, err := HashFile(filename)
	if err != nil {
		return directory, err
	}
	candidate := Candidate{SourceID: id, Title: title, SHA256: digest, Size: size, Info: mediaInfo, Profile: model.Profile{ClipSeconds: s.Config.ClipSeconds, MaxClips: ClipCount(mediaInfo.Duration, s.Config.ClipSeconds, s.Config.MaxClipsPerSource), CRF: s.Config.CRF, Preset: s.Config.Preset}, ReviewedSeconds: span}
	data, _ := json.MarshalIndent(candidate, "", "  ")
	err = os.WriteFile(filepath.Join(directory, "candidate.json"), data, 0600)
	return directory, err
}
func (s Service) ffmpeg(ctx context.Context, args ...string) error {
	base := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-protocol_whitelist", "file,pipe"}
	cmd := exec.CommandContext(ctx, s.Tools.FFmpegPath, append(base, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("media processing failed: %w: %.512s", err, stderr.String())
	}
	return nil
}
func (s Service) Publish(ctx context.Context, directory string, decision Decision) (index model.Index, err error) {
	if err = s.Validate(); err != nil {
		return index, err
	}
	raw, err := os.ReadFile(filepath.Join(directory, "candidate.json"))
	if err != nil {
		return index, err
	}
	var c Candidate
	if err = json.Unmarshal(raw, &c); err != nil {
		return index, err
	}
	if err = ValidateDecision(c, decision); err != nil {
		return index, err
	}
	if !decision.Suitable {
		return index, os.RemoveAll(directory)
	}
	digest, size, err := HashFile(filepath.Join(directory, "source.mp4"))
	if err != nil {
		return index, err
	}
	if digest != c.SHA256 || size != c.Size {
		return index, errors.New("candidate changed after review")
	}
	info, err := media.ProbeFile(ctx, s.Tools.FFprobePath, filepath.Join(directory, "source.mp4"))
	if err != nil {
		return index, err
	}
	info.Duration = UsableDuration(info)
	if !info.HasAudio || info.Duration+0.001 < c.ReviewedSeconds {
		return index, errors.New("review exceeds usable media")
	}
	c.Info = info
	approvedProfile, err := ApprovedProfile(c, s.Config.MaxClipsPerSource)
	if err != nil {
		return index, err
	}
	indexRef, err := s.Store.Reference(path.Join(s.Config.Prefix, "index.json"))
	if err != nil {
		return index, err
	}
	lockName := "slice:" + storage.DigestOf([]byte(indexRef))[:48]
	err = s.Repository.WithLock(ctx, lockName, func(repo repository.Repository) error {
		profile := approvedProfile
		profileRaw, _ := json.Marshal(profile)
		source, e := repo.Reserve(ctx, model.Source{SourceID: c.SourceID, SHA256: c.SHA256, OriginalTitle: c.Title, Basename: ShortTitle(decision.Title), ProfileJSON: string(profileRaw), ReviewReason: decision.Reason})
		if e != nil {
			return e
		}
		// Reusing a source also reuses the original encoding/clip parameters.
		if e = json.Unmarshal([]byte(source.ProfileJSON), &profile); e != nil {
			return e
		}
		if source.ClipsJSON == nil && float64(profile.MaxClips)*profile.ClipSeconds > c.ReviewedSeconds+0.001 {
			return errors.New("resume requires the original full-range reviewed candidate")
		}
		if source.ClipsJSON == nil {
			clips := []model.Clip{}
			for i := 0; i < ClipCount(c.Info.Duration, profile.ClipSeconds, profile.MaxClips); i++ {
				name := source.Basename + "_" + strconv.Itoa(i+1) + ".mp4"
				output := filepath.Join(directory, name)
				start := float64(i) * profile.ClipSeconds
				if e = s.ffmpeg(ctx, "-ss", fmt.Sprint(start), "-i", filepath.Join(directory, "source.mp4"), "-t", fmt.Sprint(profile.ClipSeconds), "-map", "0:v:0", "-map", "0:a:0", "-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2,setsar=1", "-c:v", "libx264", "-crf", strconv.Itoa(profile.CRF), "-preset", profile.Preset, "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart", "-map_metadata", "-1", output); e != nil {
					return e
				}
				info, e := media.ProbeFile(ctx, s.Tools.FFprobePath, output)
				if e != nil {
					return e
				}
				if !info.HasAudio || info.VideoDuration <= 0 || info.AudioDuration <= 0 || math.Abs(info.VideoDuration-profile.ClipSeconds) > 0.12 || math.Abs(info.AudioDuration-profile.ClipSeconds) > 0.12 || math.Abs(info.Duration-profile.ClipSeconds) > 0.12 {
					return errors.New("rendered clip has invalid duration/audio")
				}
				if e = s.ffmpeg(ctx, "-i", output, "-f", "null", "-"); e != nil {
					return e
				}
				digest, n, e := HashFile(output)
				if e != nil {
					return e
				}
				ref, e := s.Store.Reference(path.Join(s.Config.Prefix, name))
				if e != nil {
					return e
				}
				f, e := os.Open(output)
				if e != nil {
					return e
				}
				e = s.Store.PutImmutable(ctx, ref, f, n, "video/mp4", digest)
				_ = f.Close()
				if e != nil {
					return e
				}
				clips = append(clips, model.Clip{SourceID: c.SourceID, Title: source.Basename, Location: ref, Start: start, Duration: info.Duration, Size: n, SHA256: digest})
			}
			if len(clips) == 0 {
				return errors.New("source no longer has a complete clip")
			}
			data, _ := json.Marshal(clips)
			if e = repo.Ready(ctx, c.SourceID, string(data)); e != nil {
				return e
			}
		}
		rows, e := repo.Sources(ctx)
		if e != nil {
			return e
		}
		index = model.Index{Version: 1, Clips: []model.Clip{}}
		for _, row := range rows {
			var clips []model.Clip
			if e = json.Unmarshal([]byte(*row.ClipsJSON), &clips); e != nil {
				return e
			}
			index.Clips = append(index.Clips, clips...)
		}
		// Ready rows already passed full immutable-upload read-back. Rebuilding
		// this projection must not download the growing library on every append.
		// The explicit verify operation audits all object bytes; composition also
		// checks the selected resource against its frozen digest before use.
		data, _ := json.MarshalIndent(index, "", "  ")
		if e = s.Store.Put(ctx, indexRef, bytes.NewReader(data), int64(len(data)), "application/json"); e != nil {
			return e
		}
		body, e := s.Store.Open(ctx, indexRef)
		if e != nil {
			return e
		}
		actual, e := io.ReadAll(io.LimitReader(body, int64(len(data))+1))
		_ = body.Close()
		if e != nil {
			return e
		}
		if !bytes.Equal(data, actual) {
			return errors.New("library index read-back mismatch")
		}
		return nil
	})
	return index, err
}
func VerifyClip(ctx context.Context, store storage.ImmutableStore, clip model.Clip) error {
	body, err := store.Open(ctx, clip.Location)
	if err != nil {
		return err
	}
	defer body.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(body, clip.Size+1))
	if err != nil {
		return err
	}
	if n != clip.Size || hex.EncodeToString(h.Sum(nil)) != clip.SHA256 {
		return errors.New("indexed clip integrity mismatch")
	}
	return nil
}
