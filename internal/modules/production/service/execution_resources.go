package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	slices "github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/model"
)

var errManifestLeaseLost = repository.ErrComposeCommitLost

type manifestStore interface {
	FreezeComposeManifest(int64, int64, int, json.RawMessage) (json.RawMessage, error)
}
type frozenResource struct {
	Name     string              `json:"name"`
	Location string              `json:"location"`
	SHA256   string              `json:"sha256"`
	Size     int64               `json:"size"`
	Media    executor.MediaInput `json:"media"`
}
type executionManifest struct {
	Version   int              `json:"version"`
	Source    frozenResource   `json:"source"`
	Resources []frozenResource `json:"resources"`
	Timeline  json.RawMessage  `json:"timeline"`
}

func resourceFact(name, location, path string, media executor.MediaInput) (frozenResource, error) {
	f, err := os.Open(path)
	if err != nil {
		return frozenResource{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return frozenResource{}, err
	}
	if n <= 0 {
		return frozenResource{}, errors.New("empty media resource")
	}
	media.Path = ""
	return frozenResource{Name: name, Location: location, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n, Media: media}, nil
}
func verifyResource(first, next frozenResource) error {
	if first.Name != next.Name || first.Location != next.Location || first.Size != next.Size || first.SHA256 != next.SHA256 || first.Media != next.Media {
		return errors.New("frozen media resource changed")
	}
	return nil
}
func checkFrozenTimeline(a, b json.RawMessage) error {
	if !sameJSON(a, b) {
		return errors.New("frozen execution timeline changed")
	}
	return nil
}
func freezeManifest(ctx context.Context, store manifestStore, task model.ComposeTask, nodeID int64, manifest executionManifest) (executionManifest, error) {
	if err := ctx.Err(); err != nil {
		return executionManifest{}, err
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return executionManifest{}, err
	}
	frozen, err := store.FreezeComposeManifest(task.ID, nodeID, task.AttemptCount, raw)
	if err != nil {
		if errors.Is(err, errManifestLeaseLost) {
			return executionManifest{}, err
		}
		return executionManifest{}, fmt.Errorf("%w: freeze execution facts", compose.ErrGuideTemporary)
	}
	if !sameJSON(raw, frozen) {
		return executionManifest{}, errors.New("execution manifest differs from the first preparation")
	}
	var result executionManifest
	err = json.Unmarshal(frozen, &result)
	return result, err
}
func resolveFileResource(ctx context.Context, token string) (frozenResource, error) {
	svc, err := files.Wired()
	if err != nil {
		return frozenResource{}, err
	}
	f, err := svc.ResolveToken(ctx, token)
	if err != nil {
		if errors.Is(err, files.ErrNotFound) || errors.Is(err, files.ErrInvalid) || errors.Is(err, files.ErrForbidden) {
			return frozenResource{}, err
		}
		return frozenResource{}, fmt.Errorf("%w: resolve platform file", compose.ErrGuideTemporary)
	}
	if f.Type != "video" {
		return frozenResource{}, errors.New("composition resource must be a video")
	}
	return frozenResource{Location: f.ObjectKey, SHA256: f.SHA256, Size: f.SizeBytes}, nil
}

// prepareResources handles declarations, immutable facts and lease ownership.
// Rendering style and timeline math stay entirely inside the executor.
func prepareResources(ctx context.Context, deps ComposeExecutorDeps, task model.ComposeTask, nodeID int64, plan executor.Plan, inputPath, runDir string) (map[string]string, json.RawMessage, error) {
	requests, err := plan.Resources(task.TechnicalConfig)
	if err != nil {
		return nil, nil, err
	}
	paths := map[string]string{}
	frozenRun := len(task.TechnicalConfig) > 0
	var previous executionManifest
	if len(task.ExecutionManifest) > 0 {
		if json.Unmarshal(task.ExecutionManifest, &previous) != nil || previous.Version != 1 {
			return nil, nil, errors.New("invalid execution manifest")
		}
	}
	var manifest executionManifest
	if frozenRun {
		if task.RandomSeed == 0 {
			return nil, nil, errors.New("task has no random seed")
		}
		source, err := inspectResource(ctx, deps, inputPath)
		if err != nil {
			return nil, nil, err
		}
		material, found, err := deps.Materials.FindMaterial(task.MaterialID)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: read source facts", compose.ErrGuideTemporary)
		}
		if !found {
			return nil, nil, errors.New("source material missing")
		}
		fact, err := resourceFact("source", material.SourceObjectKey, inputPath, source)
		if err != nil {
			return nil, nil, err
		}
		if material.VideoSHA256 != "" && fact.SHA256 != material.VideoSHA256 || material.VideoSizeBytes != nil && fact.Size != *material.VideoSizeBytes {
			return nil, nil, errors.New("source content verification failed")
		}
		if previous.Version > 0 {
			if err = verifyResource(previous.Source, fact); err != nil {
				return nil, nil, err
			}
		}
		manifest = executionManifest{Version: 1, Source: fact, Resources: []frozenResource{}}
	}
	if previous.Version > 0 && len(previous.Resources) != len(requests) {
		return nil, nil, errors.New("frozen resource declarations changed")
	}
	names := map[string]bool{}
	for i, request := range requests {
		if request.Name == "" || filepath.Base(request.Name) != request.Name || names[request.Name] {
			return nil, nil, errors.New("invalid resource declaration")
		}
		names[request.Name] = true
		target := filepath.Join(runDir, request.Name+"-input")
		var expected frozenResource
		if previous.Version > 0 {
			expected = previous.Resources[i]
			if expected.Name != request.Name {
				return nil, nil, errors.New("frozen resource order changed")
			}
		} else if request.PoolIndexKey != "" {
			expected, err = choosePoolResource(ctx, deps.Store, request.PoolIndexKey, task.RandomSeed)
			if err != nil {
				return nil, nil, err
			}
		} else if token, owned := executor.FileResourceToken(request.URL); owned {
			resolve := deps.ResolveFileResource
			if resolve == nil {
				resolve = resolveFileResource
			}
			expected, err = resolve(ctx, token)
			if err != nil {
				return nil, nil, err
			}
		} else {
			expected.Location = request.URL
		}
		expected.Name = request.Name
		if request.PoolIndexKey != "" || len(expected.Location) > 5 && expected.Location[:5] == "s3://" {
			err = downloadStoredResource(ctx, deps.Store, expected, target)
		} else {
			fetch := deps.FetchGuide
			if fetch == nil {
				fetch = compose.DownloadGuide
			}
			err = fetch(ctx, expected.Location, target)
		}
		if err != nil {
			return nil, nil, err
		}
		paths[request.Name] = target
		if frozenRun {
			media, err := inspectResource(ctx, deps, target)
			if err != nil {
				return nil, nil, err
			}
			fact, err := resourceFact(request.Name, expected.Location, target, media)
			if err != nil {
				return nil, nil, err
			}
			if expected.SHA256 != "" && (expected.SHA256 != fact.SHA256 || expected.Size != fact.Size) {
				return nil, nil, errors.New("resource content verification failed")
			}
			if previous.Version > 0 {
				if err = verifyResource(expected, fact); err != nil {
					return nil, nil, err
				}
			}
			manifest.Resources = append(manifest.Resources, fact)
		}
	}
	if !frozenRun {
		return paths, nil, nil
	}
	source := manifest.Source.Media
	source.Path = inputPath
	mediaInputs := map[string]executor.MediaInput{}
	for _, r := range manifest.Resources {
		m := r.Media
		m.Path = paths[r.Name]
		mediaInputs[r.Name] = m
	}
	render, err := plan.Compile(executor.RenderInput{Source: source, Resources: mediaInputs, Defaults: task.TechnicalConfig, Seed: task.RandomSeed, OutputPath: filepath.Join(runDir, "output.mp4")})
	if err != nil {
		return nil, nil, err
	}
	manifest.Timeline = render.Timeline
	if previous.Version > 0 {
		if err = checkFrozenTimeline(previous.Timeline, manifest.Timeline); err != nil {
			return nil, nil, err
		}
	}
	store, ok := deps.Tasks.(manifestStore)
	if !ok {
		return nil, nil, errors.New("task store lacks immutable manifest support")
	}
	saved, err := freezeManifest(ctx, store, task, nodeID, manifest)
	return paths, saved.Timeline, err
}
func inspectResource(ctx context.Context, deps ComposeExecutorDeps, path string) (executor.MediaInput, error) {
	if deps.InspectResource != nil {
		return deps.InspectResource(ctx, path)
	}
	return executor.MediaInput{}, errors.New("resource inspector is not configured")
}
func choosePoolResource(ctx context.Context, store storage.Store, key string, seed int64) (frozenResource, error) {
	// This is a current configuration key, not an old stored logical reference.
	// Resolve it explicitly before reads so legacy material routing cannot move
	// the newly published slice index back to the retiring bucket.
	if !strings.HasPrefix(key, "s3://") {
		var err error
		key, err = storage.Reference(store, key)
		if err != nil {
			return frozenResource{}, err
		}
	}
	reader, ok := store.(interface {
		Open(context.Context, string) (io.ReadCloser, error)
	})
	if !ok {
		return frozenResource{}, errors.New("storage does not support private resource reads")
	}
	body, err := reader.Open(ctx, key)
	if err != nil {
		return frozenResource{}, fmt.Errorf("%w: read ending pool", compose.ErrGuideTemporary)
	}
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, 4*1024*1024+1))
	if err != nil {
		return frozenResource{}, fmt.Errorf("%w: read ending pool", compose.ErrGuideTemporary)
	}
	var index slices.Index
	if len(raw) > 4*1024*1024 || json.Unmarshal(raw, &index) != nil || index.Version != 1 || len(index.Clips) == 0 {
		return frozenResource{}, errors.New("ending pool is empty or invalid")
	}
	sort.Slice(index.Clips, func(i, j int) bool { return index.Clips[i].Location < index.Clips[j].Location })
	clip := index.Clips[rand.New(rand.NewSource(seed)).Intn(len(index.Clips))]
	if clip.Location == "" || len(clip.SHA256) != 64 || clip.Size <= 0 {
		return frozenResource{}, errors.New("ending pool resource is invalid")
	}
	return frozenResource{Location: clip.Location, SHA256: clip.SHA256, Size: clip.Size}, nil
}
func downloadStoredResource(ctx context.Context, store storage.Store, fact frozenResource, target string) error {
	reader, ok := store.(interface {
		Open(context.Context, string) (io.ReadCloser, error)
	})
	if !ok {
		return errors.New("storage does not support private resource reads")
	}
	info, err := store.Stat(ctx, fact.Location)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("%w: resource storage", compose.ErrGuideTemporary)
	}
	if info.Size != fact.Size {
		return errors.New("resource content verification failed")
	}
	body, err := reader.Open(ctx, fact.Location)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return err
		}
		return fmt.Errorf("%w: resource storage", compose.ErrGuideTemporary)
	}
	defer body.Close()
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(file, h), io.LimitReader(body, fact.Size+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("%w: resource download", compose.ErrGuideTemporary)
	}
	if closeErr != nil {
		return closeErr
	}
	if n != fact.Size || hex.EncodeToString(h.Sum(nil)) != fact.SHA256 {
		return errors.New("resource content verification failed")
	}
	return nil
}
