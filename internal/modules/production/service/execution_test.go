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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/compose"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

const fakeSourceKey = "materials/42/source.mp4"

var fakeSourceBytes = []byte("fake-source-video-bytes")

// lifecycleFake records every lifecycle write; each switch turns one write
// into a refusal so a test can name the branch it exercises.
type lifecycleFake struct {
	find         []model.ComposeTask // popped from the front by Find
	commitErr    error
	progressOK   bool
	releaseOK    bool
	failOK       bool
	progress     []string
	released     []string
	failed       []string
	committed    *model.CompositeOutput
	committedKey string
	ensureCalled []int64
	cancelOnFind int // after this many Find calls, report CancelRequested
	findCalls    int
}

func (f *lifecycleFake) RenewComposeLease(int64, int64, int, time.Duration) (bool, error) {
	return true, nil
}

func (f *lifecycleFake) ReportComposeProgress(_, _ int64, _ int, stage string, progress int) (bool, error) {
	f.progress = append(f.progress, fmt.Sprintf("%s:%d", stage, progress))
	return f.progressOK, nil
}

func (f *lifecycleFake) CommitComposeOutput(out model.CompositeOutput, key string, _ int64, _ int) (model.ComposeOutputCommit, error) {
	if f.commitErr != nil {
		return model.ComposeOutputCommit{}, f.commitErr
	}
	copy := out
	f.committed = &copy
	f.committedKey = key
	return model.ComposeOutputCommit{OutputID: 55, TaskID: out.TaskID}, nil
}

func (f *lifecycleFake) ReleaseComposeForRetry(_, _ int64, _ int, code, message string) (bool, error) {
	f.released = append(f.released, code+"|"+message)
	return f.releaseOK, nil
}

func (f *lifecycleFake) FailComposeTask(_, _ int64, _ int, code, message string) (bool, error) {
	f.failed = append(f.failed, code+"|"+message)
	return f.failOK, nil
}

func (f *lifecycleFake) Find(int64) (model.ComposeTask, bool, error) {
	f.findCalls++
	if len(f.find) > 0 {
		item := f.find[0]
		f.find = f.find[1:]
		return item, true, nil
	}
	return model.ComposeTask{CancelRequested: f.cancelOnFind > 0 && f.findCalls > f.cancelOnFind}, true, nil
}

type materialsFake struct{ byID map[int64]model.Material }

func (m materialsFake) FindMaterial(id int64) (model.Material, bool, error) {
	item, ok := m.byID[id]
	return item, ok, nil
}

// storeFake is an in-memory storage.Store whose presigned URLs are served by
// the same map, so upload readbacks exercise the real HTTP path.
type storeFake struct {
	objects map[string][]byte
	statErr map[string]error
}

func newStoreFake() *storeFake {
	return &storeFake{objects: map[string][]byte{}, statErr: map[string]error{}}
}

func (s *storeFake) PresignGet(_ context.Context, key string, _ time.Duration) (storage.Grant, error) {
	return storage.Grant{URL: "mem://" + key, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (s *storeFake) Stat(_ context.Context, key string) (storage.ObjectInfo, error) {
	if err := s.statErr[key]; err != nil {
		return storage.ObjectInfo{}, err
	}
	body, ok := s.objects[key]
	if !ok {
		return storage.ObjectInfo{}, errors.New("no such object " + key)
	}
	return storage.ObjectInfo{Key: key, Size: int64(len(body))}, nil
}

func (s *storeFake) Put(_ context.Context, key string, body io.Reader, size int64, _ string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return fmt.Errorf("put %s: size %d != declared %d", key, len(data), size)
	}
	s.objects[key] = data
	return nil
}

func (s *storeFake) Copy(_ context.Context, sourceKey, destinationKey string) error {
	body, ok := s.objects[sourceKey]
	if !ok {
		return errors.New("no such object " + sourceKey)
	}
	s.objects[destinationKey] = append([]byte(nil), body...)
	return nil
}

func (s *storeFake) Remove(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

func memClient(store *storeFake) *http.Client {
	return &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		key := strings.TrimPrefix(request.URL.String(), "mem://")
		body, ok := store.objects[key]
		if !ok {
			return nil, errors.New("no such object " + key)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}, nil
	})}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// runnerStub writes a deterministic output file and records the request.
type runnerStub struct {
	output   []byte
	requests []compose.Request
}

func (r *runnerStub) Run(_ context.Context, runDir *compose.RunDir, request compose.Request) (compose.Output, error) {
	r.requests = append(r.requests, request)
	path := runDir.Path() + "/output.mp4"
	if err := os.WriteFile(path, r.output, 0o644); err != nil {
		return compose.Output{}, err
	}
	return compose.Output{Path: path, DurationMS: 900}, nil
}

// killedRunner stands in for an FFmpeg process the worker terminated: it runs
// until the context ends, then reports the context error, which is what the
// real runner does when the process dies with its context.
type killedRunner struct{ started chan struct{} }

func (k killedRunner) Run(ctx context.Context, _ *compose.RunDir, _ compose.Request) (compose.Output, error) {
	if k.started != nil {
		close(k.started)
	}
	<-ctx.Done()
	return compose.Output{}, ctx.Err()
}

func attemptFixture(t *testing.T) (ComposeExecutorDeps, *lifecycleFake, *storeFake, *runnerStub, model.ComposeTask, *compose.RunDir) {
	t.Helper()
	registry, err := executor.NewRegistry(executor.BasicCutV1())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	store := newStoreFake()
	store.objects[fakeSourceKey] = fakeSourceBytes
	lifecycle := &lifecycleFake{progressOK: true, releaseOK: true, failOK: true}
	materials := materialsFake{byID: map[int64]model.Material{
		42: {ID: 42, TeamID: 7, VideoStatus: model.VideoReady, SourceObjectKey: fakeSourceKey},
	}}
	runner := &runnerStub{output: []byte("composed-output-bytes")}
	workspace, err := compose.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	runDir, err := workspace.NewRun("attempt-1")
	if err != nil {
		t.Fatalf("run dir: %v", err)
	}
	t.Cleanup(func() { _ = runDir.Release() })
	deps := ComposeExecutorDeps{
		Tasks: lifecycle, Materials: materials, Registry: registry, Store: store,
		Runner: runner, GrantTTL: time.Minute, Client: memClient(store),
	}
	task := model.ComposeTask{
		ID: 77, TeamID: 7, GameID: "delta", TriggerType: "manual", MaterialID: 42, MaterialUsageID: 9,
		StrategyID: 3, TemplateCode: "basic_cut", TemplateVersion: 1,
		ConfigSnapshot: json.RawMessage(`{"start_seconds":0,"end_seconds":1}`),
		Status:         model.ComposeTaskRunning, ClaimedNodeID: nodePtr(9),
		AttemptCount: 1, MaxAttempts: 3, CreatedBy: 11,
	}
	return deps, lifecycle, store, runner, task, runDir
}

func nodePtr(id int64) *int64 { return &id }

func TestAttemptRunsAndCommitsVerifiedOutput(t *testing.T) {
	deps, lifecycle, store, runner, task, runDir := attemptFixture(t)
	outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir)
	if outcome != ComposeAttemptSuccess {
		t.Fatalf("outcome=%s progress=%v failed=%v released=%v", outcome, lifecycle.progress, lifecycle.failed, lifecycle.released)
	}
	if len(runner.requests) != 1 || string(runner.requests[0].Params) != `{"start_seconds":0,"end_seconds":1}` {
		t.Fatalf("runner must execute the snapshot, got %+v", runner.requests)
	}
	staged, err := os.ReadFile(runner.requests[0].InputPath)
	if err != nil || !bytes.Equal(staged, fakeSourceBytes) {
		t.Fatalf("runner input must be the downloaded source bytes: err=%v len=%d", err, len(staged))
	}
	if lifecycle.committed == nil {
		t.Fatal("no output committed")
	}
	out := lifecycle.committed
	if out.TaskID != 77 || out.MaterialID != 42 || out.MaterialUsageID != 9 || out.TemplateCode != "basic_cut" || out.TemplateVersion != 1 {
		t.Fatalf("committed=%+v", out)
	}
	if out.ObjectKey != "compose-outputs/77/output.mp4" || out.PublishStatus != "pending_publish" {
		t.Fatalf("committed=%+v key=%s", out, lifecycle.committedKey)
	}
	sum := sha256.Sum256([]byte("composed-output-bytes"))
	if out.SHA256 != hex.EncodeToString(sum[:]) || out.FileSizeBytes != int64(len("composed-output-bytes")) {
		t.Fatalf("storage facts drifted: %+v", out)
	}
	if string(out.Media) != `{"duration_ms":900}` {
		t.Fatalf("media=%s", out.Media)
	}
	if _, ok := store.objects["compose-outputs/77/output.mp4"]; !ok {
		t.Fatal("formal object missing")
	}
	if _, ok := store.objects["compose-staging/77/output.mp4"]; ok {
		t.Fatal("staging object survived the commit")
	}
	want := []string{"validating_input:10", "cutting:40", "uploading:80", "committing:90"}
	if strings.Join(lifecycle.progress, ",") != strings.Join(want, ",") {
		t.Fatalf("progress=%v", lifecycle.progress)
	}
	if len(lifecycle.failed) != 0 || len(lifecycle.released) != 0 {
		t.Fatalf("unexpected writes: failed=%v released=%v", lifecycle.failed, lifecycle.released)
	}
}

func TestGuideAttemptProducesVerifiedOutputFromFrozenAddress(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	dir := t.TempDir()
	clip := func(name, color string) []byte {
		t.Helper()
		path := filepath.Join(dir, name)
		command := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c="+color+":s=320x240:r=30", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "2", "-c:v", "libx264", "-c:a", "aac", "-y", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("make clip: %v %s", err, output)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	source, guide := clip("source.mp4", "red"), clip("guide.mp4", "blue")
	store := newStoreFake()
	store.objects[fakeSourceKey] = source
	registry, err := executor.NewRegistry(executor.DeployedDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := &lifecycleFake{progressOK: true, releaseOK: true, failOK: true}
	workspace, err := compose.NewWorkspace(filepath.Join(dir, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	runDir, err := workspace.NewRun("guide-attempt")
	if err != nil {
		t.Fatal(err)
	}
	defer runDir.Release()
	address := "https://media.example.com/guide.mp4"
	deps := ComposeExecutorDeps{
		Tasks: lifecycle, Materials: materialsFake{byID: map[int64]model.Material{42: {ID: 42, TeamID: 7, VideoStatus: model.VideoReady, SourceObjectKey: fakeSourceKey}}},
		Registry: registry, Store: store, Runner: &compose.Runner{FFmpegPath: ffmpeg, FFprobePath: ffprobe, Registry: registry}, GrantTTL: time.Minute, Client: memClient(store),
		FetchGuide: func(_ context.Context, got, path string) error {
			if got != address {
				return fmt.Errorf("address changed: %s", got)
			}
			return os.WriteFile(path, guide, 0o600)
		},
	}
	task := model.ComposeTask{ID: 77, TeamID: 7, GameID: "delta", MaterialID: 42, MaterialUsageID: 9, StrategyID: 3, TemplateCode: "guide_insert", TemplateVersion: 1,
		ConfigSnapshot: json.RawMessage(`{"guide_video_url":"https://media.example.com/guide.mp4","insert_seconds":1,"guide_duration_seconds":0,"max_duration_seconds":0}`), Status: model.ComposeTaskRunning, ClaimedNodeID: nodePtr(9), CreatedBy: 11}
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptSuccess {
		t.Fatalf("outcome=%s failed=%v released=%v", outcome, lifecycle.failed, lifecycle.released)
	}
	formal := store.objects["compose-outputs/77/output.mp4"]
	if len(formal) == 0 || lifecycle.committed == nil || lifecycle.committed.TemplateCode != "guide_insert" {
		t.Fatal("verified formal output missing")
	}
	if output, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1", filepath.Join(runDir.Path(), "output.mp4")).CombinedOutput(); err != nil || !bytes.Contains(output, []byte("4.0")) {
		t.Fatalf("output probe=%s err=%v", output, err)
	}
}

func TestAttemptWaitsWhenSourceNotReady(t *testing.T) {
	deps, lifecycle, _, runner, task, runDir := attemptFixture(t)
	deps.Materials = materialsFake{byID: map[int64]model.Material{
		42: {ID: 42, VideoStatus: model.VideoNotDownloaded},
	}}
	var ensureTasks []model.ComposeTask
	ensure := []int64{}
	deps.EnsureInputPrepared = func(_ context.Context, requestor model.ComposeTask, materialID int64) error {
		ensure = append(ensure, materialID)
		ensureTasks = append(ensureTasks, requestor)
		return nil
	}
	outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir)
	if outcome != ComposeAttemptWaitInput {
		t.Fatalf("outcome=%s", outcome)
	}
	if len(ensure) != 1 || ensure[0] != 42 {
		t.Fatalf("ensure=%v", ensure)
	}
	if len(ensureTasks) != 1 || ensureTasks[0].ID != 77 || ensureTasks[0].CreatedBy != 11 {
		t.Fatalf("the hook must see the task so preparation can be requested in its name: %+v", ensureTasks)
	}
	if !strings.Contains(strings.Join(lifecycle.progress, ","), "waiting_input:20") {
		t.Fatalf("progress=%v", lifecycle.progress)
	}
	if len(runner.requests) != 0 || lifecycle.committed != nil || len(lifecycle.failed) != 0 {
		t.Fatalf("wait must not execute or write terminal state: committed=%v failed=%v", lifecycle.committed, lifecycle.failed)
	}
}

func TestAttemptFailsWhenMaterialRowIsGone(t *testing.T) {
	deps, lifecycle, _, _, task, runDir := attemptFixture(t)
	deps.Materials = materialsFake{byID: map[int64]model.Material{}}
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptFailed {
		t.Fatalf("outcome=%s", outcome)
	}
	if len(lifecycle.failed) != 1 || !strings.HasPrefix(lifecycle.failed[0], composeErrInputMissing+"|") {
		t.Fatalf("failed=%v", lifecycle.failed)
	}
}

func TestAttemptFailsBusinessWhenSourcePreparationFailed(t *testing.T) {
	deps, lifecycle, _, runner, task, runDir := attemptFixture(t)
	deps.Materials = materialsFake{byID: map[int64]model.Material{
		42: {ID: 42, VideoStatus: model.VideoFailed},
	}}
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptFailed {
		t.Fatalf("outcome=%s", outcome)
	}
	if len(lifecycle.failed) != 1 || !strings.HasPrefix(lifecycle.failed[0], composeErrInputInvalid+"|") {
		t.Fatalf("failed=%v", lifecycle.failed)
	}
	if len(lifecycle.released) != 0 || len(runner.requests) != 0 {
		t.Fatalf("business failure must not retry or run: released=%v", lifecycle.released)
	}
}

func TestAttemptRetriesTransientStorageAndFailsWhenExhausted(t *testing.T) {
	deps, lifecycle, store, _, task, runDir := attemptFixture(t)
	store.statErr[fakeSourceKey] = errors.New("storage unreachable")
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptRetry {
		t.Fatalf("outcome=%s", outcome)
	}
	if len(lifecycle.released) != 1 || !strings.HasPrefix(lifecycle.released[0], composeErrStorageError+"|") {
		t.Fatalf("released=%v", lifecycle.released)
	}
	lifecycle2 := &lifecycleFake{progressOK: true, releaseOK: true, failOK: true}
	deps.Tasks = lifecycle2
	exhausted := task
	exhausted.AttemptCount = 3
	if outcome := RunComposeAttempt(context.Background(), deps, exhausted, 9, runDir); outcome != ComposeAttemptFailed {
		t.Fatalf("exhausted outcome=%s", outcome)
	}
	if len(lifecycle2.failed) != 1 || !strings.Contains(lifecycle2.failed[0], "attempts exhausted after 3") {
		t.Fatalf("failed=%v", lifecycle2.failed)
	}
}

func TestAttemptDoesNotRetryMissingSourceObject(t *testing.T) {
	deps, lifecycle, store, _, task, runDir := attemptFixture(t)
	store.statErr[fakeSourceKey] = storage.ErrObjectNotFound
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptFailed {
		t.Fatalf("missing source outcome=%s", outcome)
	}
	if len(lifecycle.released) != 0 || len(lifecycle.failed) != 1 || !strings.HasPrefix(lifecycle.failed[0], composeErrInputMissing+"|") {
		t.Fatalf("missing source must be terminal: failed=%v released=%v", lifecycle.failed, lifecycle.released)
	}
}

func TestAttemptDoesNotRequeueAfterContextCancellation(t *testing.T) {
	deps, lifecycle, store, _, task, runDir := attemptFixture(t)
	store.statErr[fakeSourceKey] = context.Canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := RunComposeAttempt(ctx, deps, task, 9, runDir); got != ComposeAttemptLeaseLost {
		t.Fatalf("cancelled attempt outcome=%s", got)
	}
	if len(lifecycle.released) != 0 || len(lifecycle.failed) != 0 {
		t.Fatalf("cancelled attempt wrote retry/failure: released=%v failed=%v", lifecycle.released, lifecycle.failed)
	}
}

func TestAttemptClassifiesGuideDownloadFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want ComposeAttemptOutcome
	}{
		{"temporary network", fmt.Errorf("fetch: %w", compose.ErrGuideTemporary), ComposeAttemptRetry},
		{"expired link", errors.New("guide HTTP 404"), ComposeAttemptFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, lifecycle, _, _, task, runDir := attemptFixture(t)
			task.TemplateCode = "guide_insert"
			task.ConfigSnapshot = json.RawMessage(`{"guide_video_url":"https://media.example.com/guide.mp4","insert_seconds":1,"guide_duration_seconds":0,"max_duration_seconds":0}`)
			registry, err := executor.NewRegistry(executor.DeployedDefinitions()...)
			if err != nil {
				t.Fatal(err)
			}
			deps.Registry = registry
			deps.FetchGuide = func(context.Context, string, string) error { return tc.err }
			if got := RunComposeAttempt(context.Background(), deps, task, 9, runDir); got != tc.want {
				t.Fatalf("outcome=%s want=%s", got, tc.want)
			}
			if tc.want == ComposeAttemptRetry && len(lifecycle.released) != 1 {
				t.Fatalf("temporary download not requeued: %v", lifecycle.released)
			}
			if tc.want == ComposeAttemptFailed && len(lifecycle.failed) != 1 {
				t.Fatalf("permanent download not failed: %v", lifecycle.failed)
			}
		})
	}
}

func TestAttemptFailsOnSnapshotRejectionBeforeAnyBytesMove(t *testing.T) {
	deps, lifecycle, _, runner, task, runDir := attemptFixture(t)
	task.ConfigSnapshot = json.RawMessage(`{"start_seconds":5}`)
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptFailed {
		t.Fatalf("outcome=%s", outcome)
	}
	if len(lifecycle.failed) != 1 || !strings.HasPrefix(lifecycle.failed[0], composeErrTemplateUnavailable+"|") {
		t.Fatalf("failed=%v", lifecycle.failed)
	}
	if len(runner.requests) != 0 {
		t.Fatal("a rejected snapshot must never reach the runner")
	}
}

func TestAttemptCancelsWhenRequestedBeforeCommit(t *testing.T) {
	deps, lifecycle, store, _, task, runDir := attemptFixture(t)
	lifecycle.cancelOnFind = 1 // first Find (after input gate) reports the request
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptCancelled {
		t.Fatalf("outcome=%s", outcome)
	}
	if lifecycle.committed != nil {
		t.Fatal("a cancelled attempt must not commit")
	}
	if _, ok := store.objects["compose-staging/77/output.mp4"]; ok {
		t.Fatal("staging object must not exist")
	}
}

func TestAttemptCleansOrphansWhenCommitLosesTheRow(t *testing.T) {
	deps, lifecycle, store, _, task, runDir := attemptFixture(t)
	lifecycle.commitErr = repository.ErrComposeCommitLost
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptLeaseLost {
		t.Fatalf("outcome=%s", outcome)
	}
	for _, key := range []string{"compose-outputs/77/output.mp4", "compose-staging/77/output.mp4"} {
		if _, ok := store.objects[key]; ok {
			t.Fatalf("orphan %s survived", key)
		}
	}
	if lifecycle.committed != nil {
		t.Fatal("no output may be reported as committed")
	}
}

func TestAttemptReportsLeaseLostWhenProgressIsRefused(t *testing.T) {
	deps, lifecycle, _, _, task, runDir := attemptFixture(t)
	lifecycle.progressOK = false
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptLeaseLost {
		t.Fatalf("outcome=%s", outcome)
	}
	if lifecycle.committed != nil || len(lifecycle.failed) != 0 || len(lifecycle.released) != 0 {
		t.Fatalf("lost lease must write nothing: committed=%v failed=%v", lifecycle.committed, lifecycle.failed)
	}
}

func TestAttemptUploadRejectsTamperedReadback(t *testing.T) {
	deps, lifecycle, _, _, task, runDir := attemptFixture(t)
	// A readback that differs from the uploaded bytes must fail the upload,
	// so the tampered-object case can never reach the commit transaction.
	tamper := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		key := strings.TrimPrefix(request.URL.String(), "mem://")
		if strings.HasPrefix(key, "compose-staging/") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("tampered-bytes"))), Header: http.Header{}}, nil
		}
		body, ok := deps.Store.(*storeFake).objects[key]
		if !ok {
			return nil, errors.New("no such object " + key)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}, nil
	})
	deps.Client = &http.Client{Transport: tamper}
	if outcome := RunComposeAttempt(context.Background(), deps, task, 9, runDir); outcome != ComposeAttemptFailed {
		t.Fatalf("outcome=%s", outcome)
	}
	if lifecycle.committed != nil {
		t.Fatal("unverified bytes must not commit")
	}
	if len(lifecycle.released) != 0 || len(lifecycle.failed) != 1 || !strings.HasPrefix(lifecycle.failed[0], composeErrExecutionFailed+"|") {
		t.Fatalf("invalid output must not auto retry: failed=%v released=%v", lifecycle.failed, lifecycle.released)
	}
}

// M4-C5 T-04: a run the worker terminated (cancel request or task timeout)
// must be classified by why the context died, not by FFmpeg's last words.

func TestAttemptClassifiesTerminatedRunAsCancelledWhenRequested(t *testing.T) {
	deps, _, _, _, task, runDir := attemptFixture(t)
	// Find #1 is the pre-cut cancel check (no request yet); Find #2 happens
	// after the runner died and must see the request that stopped the process.
	killed := &lifecycleFake{progressOK: true, releaseOK: true, failOK: true, find: []model.ComposeTask{{CancelRequested: false}, {CancelRequested: true}}}
	deps.Tasks = killed
	deps.Runner = killedRunner{started: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan ComposeAttemptOutcome, 1)
	go func() {
		done <- RunComposeAttempt(ctx, deps, task, 9, runDir)
	}()
	select {
	case <-deps.Runner.(killedRunner).started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner never started")
	}
	cancel()
	if outcome := <-done; outcome != ComposeAttemptCancelled {
		t.Fatalf("outcome=%s", outcome)
	}
	if len(killed.failed) != 0 || len(killed.released) != 0 || killed.committed != nil {
		t.Fatalf("a cancelled termination is a controlled stop, not a failure: failed=%v released=%v", killed.failed, killed.released)
	}
}

func TestAttemptClassifiesTerminatedRunAsTransientTimeout(t *testing.T) {
	deps, lifecycle, _, _, task, runDir := attemptFixture(t)
	deps.Runner = killedRunner{started: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if outcome := RunComposeAttempt(ctx, deps, task, 9, runDir); outcome != ComposeAttemptRetry {
		t.Fatalf("outcome=%s", outcome)
	}
	if len(lifecycle.released) != 1 || !strings.HasPrefix(lifecycle.released[0], composeErrTaskTimeout+"|") {
		t.Fatalf("a timed-out task is transient and requeued: released=%v", lifecycle.released)
	}
	if lifecycle.committed != nil || len(lifecycle.failed) != 0 {
		t.Fatalf("no commit and no business failure on a timeout: committed=%v failed=%v", lifecycle.committed, lifecycle.failed)
	}
}
