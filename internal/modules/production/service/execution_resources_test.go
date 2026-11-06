package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	slices "github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/model"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFrozenResourceRejectsChangedBytesAndTimeline(t *testing.T) {
	p := filepath.Join(t.TempDir(), "video")
	os.WriteFile(p, []byte("original video"), 0600)
	first, err := resourceFact("intro", "s3://wt-media/file-ku/files/3_intro.mp4", p, executor.MediaInput{Duration: 8, Width: 1280, Height: 720, HasAudio: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyResource(first, first); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte("replaced video"), 0600)
	next, err := resourceFact("intro", first.Location, p, first.Media)
	if err != nil {
		t.Fatal(err)
	}
	if verifyResource(first, next) == nil {
		t.Fatal("changed content accepted on retry")
	}
	if err = checkFrozenTimeline(json.RawMessage(`{"duration":20}`), json.RawMessage(`{"duration":21}`)); err == nil {
		t.Fatal("changed timeline accepted")
	}
}

type manifestLifecycle struct {
	*lifecycleFake
	manifestMemory
}
type resourceStore struct {
	*storeFake
	opens   []string
	statErr error
}

func (s *resourceStore) Reference(key string) (string, error) { return "s3://wt-media/" + key, nil }

func TestPoolIndexUsesCurrentBucketWhileLegacyReadsRemainSeparate(t *testing.T) {
	objects := &resourceStore{storeFake: newStoreFake()}
	objects.objects["s3://wt-media/pool/index.json"] = []byte(`{"version":1,"clips":[{"location":"s3://wt-media/pool/one.mp4","size":9,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`)
	got, err := choosePoolResource(context.Background(), objects, "pool/index.json", 42)
	if err != nil || got.Location != "s3://wt-media/pool/one.mp4" {
		t.Fatalf("pool routed through legacy bucket: %+v %v", got, err)
	}
	if len(objects.opens) != 1 || objects.opens[0] != "s3://wt-media/pool/index.json" {
		t.Fatalf("opens=%v", objects.opens)
	}
}

func (s *resourceStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	s.opens = append(s.opens, key)
	b, ok := s.objects[key]
	if !ok {
		return nil, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (s *resourceStore) Stat(ctx context.Context, key string) (storage.ObjectInfo, error) {
	if s.statErr != nil {
		return storage.ObjectInfo{}, s.statErr
	}
	return s.storeFake.Stat(ctx, key)
}

func TestPreparedRetryKeepsPoolChoiceAndRejectsChangedObject(t *testing.T) {
	deps, life, store, _, task, run := attemptFixture(t)
	ms := &manifestLifecycle{lifecycleFake: life, manifestMemory: manifestMemory{owned: true}}
	objects := &resourceStore{storeFake: store}
	deps.Tasks = ms
	deps.Store = objects
	raw := json.RawMessage(`{"intro_video_url":"https://example.com/i.mp4","ad_video_url":"https://example.com/a.mp4","start_seconds":0,"end_seconds":0,"max_duration_seconds":180,"ad_start_min_seconds":10,"ad_start_max_seconds":15,"intro_pip_width":0.9,"ad_pip_width":0.92,"transition_seconds":0.2}`)
	plan, err := executor.DynamicPIPEndingV1().Executor.ValidateParams(raw)
	if err != nil {
		t.Fatal(err)
	}
	task.ConfigSnapshot = raw
	task.RandomSeed = 77
	task.TechnicalConfig = json.RawMessage(`{"width":1280,"height":720,"fps":30,"crf":20,"preset":"medium","blur":20,"brightness":-0.08,"shadow_blur":8,"shadow_opacity":0.35,"shadow_offset":8,"tail_seconds":6,"tail_fade":0.35,"ending_index_key":"pool/index.json","filter_threads":1}`)
	input := filepath.Join(run.Path(), "source-input")
	os.WriteFile(input, fakeSourceBytes, 0600)
	deps.InspectResource = func(_ context.Context, p string) (executor.MediaInput, error) {
		d := 100.0
		switch filepath.Base(p) {
		case "intro-input":
			d = 8
		case "ad-input":
			d = 10
		case "ending-input":
			d = 6
		}
		return executor.MediaInput{Path: p, Duration: d, Width: 1280, Height: 720, HasAudio: true}, nil
	}
	deps.FetchGuide = func(_ context.Context, _ string, p string) error { return os.WriteFile(p, []byte("guide video"), 0600) }
	tail := []byte("tail video")
	sum := sha256.Sum256(tail)
	location := "s3://wt-media/file-ku/随机视频切片/tail_1.mp4"
	store.objects[location] = tail
	index, _ := json.Marshal(slices.Index{Version: 1, Clips: []slices.Clip{{Location: location, Size: int64(len(tail)), SHA256: hex.EncodeToString(sum[:])}}})
	store.objects["s3://wt-media/pool/index.json"] = index
	_, first, err := prepareResources(context.Background(), deps, task, 9, plan, input, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task.ExecutionManifest = ms.raw
	delete(store.objects, "s3://wt-media/pool/index.json")
	objects.opens = nil
	_, second, err := prepareResources(context.Background(), deps, task, 9, plan, input, t.TempDir())
	if err != nil || !sameJSON(first, second) {
		t.Fatalf("retry changed choice/timeline: %v", err)
	}
	for _, key := range objects.opens {
		if key == "pool/index.json" {
			t.Fatal("retry reread pool")
		}
	}
	objects.statErr = errors.New("storage temporarily down")
	_, _, err = prepareResources(context.Background(), deps, task, 9, plan, input, t.TempDir())
	if !errors.Is(err, compose.ErrGuideTemporary) {
		t.Fatalf("temporary failure became permanent: %v", err)
	}
	objects.statErr = nil
	store.objects[location] = []byte("other data")
	if _, _, err = prepareResources(context.Background(), deps, task, 9, plan, input, t.TempDir()); err == nil {
		t.Fatal("changed pool video accepted")
	}
}

type manifestMemory struct {
	raw   json.RawMessage
	owned bool
}

func (m *manifestMemory) FreezeComposeManifest(_ int64, _ int64, _ int, raw json.RawMessage) (json.RawMessage, error) {
	if !m.owned {
		return nil, errManifestLeaseLost
	}
	if m.raw == nil {
		m.raw = append(json.RawMessage(nil), raw...)
	}
	return m.raw, nil
}
func TestManifestRequiresCurrentLeaseAndReusesFirstFacts(t *testing.T) {
	m := &manifestMemory{owned: true}
	task := model.ComposeTask{ID: 1, AttemptCount: 1}
	first := executionManifest{Version: 1, Timeline: json.RawMessage(`{"duration":20}`)}
	got, err := freezeManifest(context.Background(), m, task, 9, first)
	if err != nil || string(got.Timeline) != string(first.Timeline) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	first.Timeline = json.RawMessage(`{"duration":21}`)
	if _, err = freezeManifest(context.Background(), m, task, 9, first); err == nil {
		t.Fatal("replaced first timeline")
	}
	m.owned = false
	if _, err = freezeManifest(context.Background(), m, task, 9, first); err == nil {
		t.Fatal("lost lease accepted")
	}
}

func TestMissingResourceInspectorReturnsConfigurationError(t *testing.T) {
	_, err := inspectResource(context.Background(), ComposeExecutorDeps{}, "unused")
	if err == nil {
		t.Fatal("missing inspector must return an error before processing a claimed task")
	}
}
