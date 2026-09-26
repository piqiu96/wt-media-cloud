package jobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/sourcefile"
	"github.com/wt-media/wt-media-cloud/internal/infra/media"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	transfermodel "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	transferrepo "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/repository"
	producerepo "github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// The matrix below is this CHG's failure-injection evidence, so it is written to
// be read as a list of the ways a preparation can go wrong rather than as a set
// of unit tests. Two things are asserted after every row, whichever row it was:
//
//   - the material was never told it is ready, and
//   - no object was written at a formal key.
//
// Those two are the invariant the whole file exists to protect. A row that fails
// its own specific assertion is a bug in one branch; a row that leaves the
// material `ready` without verified bytes is the failure this feature cannot have.

const (
	testWorkerID  = "cloud-worker:test"
	testTaskID    = "task-1"
	testMaterial  = int64(42)
	testTeamID    = identity.TeamID(7)
	testContentID = int64(7123456789012345678)
)

// The address is a signed URL in production, so the fixture is one too — reserved
// by RFC 6761 so that it can never resolve, and carrying a marker every row asserts
// never reaches a log line, a failure message or a stored payload.
const testAddress = "https://cdn.example.invalid/media/42.mp4?signature=DO-NOT-LOG-ME"

var testNow = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

// eventLog records what happened, in order, across every fake. Ordering is a
// correctness property here — the facts are written after the object is verified,
// and the waiters are released before the task is closed — and a per-fake recorder
// could not see it.
type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

func (l *eventLog) indexOf(event string) int {
	for index, recorded := range l.all() {
		if recorded == event {
			return index
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// The fakes, one per seam.
// ---------------------------------------------------------------------------

type fakeMaterials struct {
	events *eventLog

	material  productionservice.PreparationSource
	sourceErr error

	readyFacts []producerepo.VideoFacts
	readyErr   error

	failed    []string
	failedErr error
}

func (f *fakeMaterials) ResolvePreparationSource(teamID identity.TeamID, materialID int64) (productionservice.PreparationSource, error) {
	f.events.add("resolve")
	if f.sourceErr != nil {
		return productionservice.PreparationSource{}, f.sourceErr
	}
	if teamID != f.material.TeamID || materialID != f.material.MaterialID {
		return productionservice.PreparationSource{}, fmt.Errorf("no material %d in team %d", materialID, teamID)
	}
	return f.material, nil
}

// MarkVideoReady records the event only when the write lands. The event means "the
// material now says it is ready", which is the thing the matrix's invariant is
// about, so a write that failed must not leave one behind.
func (f *fakeMaterials) MarkVideoReady(teamID identity.TeamID, materialID int64, facts producerepo.VideoFacts) error {
	if f.readyErr != nil {
		return f.readyErr
	}
	f.events.add("ready")
	f.readyFacts = append(f.readyFacts, facts)
	return nil
}

func (f *fakeMaterials) MarkVideoFailed(teamID identity.TeamID, materialID int64, message string) error {
	f.events.add("failed")
	f.failed = append(f.failed, message)
	return f.failedErr
}

type fakeTransfers struct {
	events *eventLog

	claimed    transfermodel.Task
	claimFound bool
	claimErr   error

	heartbeatOK  bool
	heartbeatErr error
	heartbeats   int

	handoverErr    error
	handoverFacts  []transferrepo.DependencyFacts
	handoverCalled int

	completeOK       bool
	completeErr      error
	completeCalled   int
	completionInputs []transferrepo.CompletionInput

	failTaskOK    bool
	failTaskErr   error
	failInputs    []transferrepo.FailureInput
	dependentsErr error

	current transfermodel.Task
	getErr  error
}

func (f *fakeTransfers) ClaimCloudTask(workerID string, now time.Time, lease time.Duration) (transfermodel.Task, bool, error) {
	f.events.add("claim")
	return f.claimed, f.claimFound, f.claimErr
}

func (f *fakeTransfers) GetTask(taskID string) (transfermodel.Task, error) {
	f.events.add("get")
	return f.current, f.getErr
}

func (f *fakeTransfers) HeartbeatTask(taskID, nodeID string, now time.Time, lease time.Duration) (bool, error) {
	f.events.add("heartbeat")
	f.heartbeats++
	return f.heartbeatOK, f.heartbeatErr
}

func (f *fakeTransfers) HandOverDependencies(prepareTaskID string, facts transferrepo.DependencyFacts, now time.Time) (int64, error) {
	f.events.add("handover")
	f.handoverCalled++
	if f.handoverErr != nil {
		return 0, f.handoverErr
	}
	f.handoverFacts = append(f.handoverFacts, facts)
	return 1, nil
}

func (f *fakeTransfers) CompleteTask(input transferrepo.CompletionInput, now time.Time) (bool, error) {
	f.events.add("complete")
	f.completeCalled++
	if f.completeErr != nil {
		return false, f.completeErr
	}
	f.completionInputs = append(f.completionInputs, input)
	return f.completeOK, nil
}

func (f *fakeTransfers) FailTask(input transferrepo.FailureInput, now time.Time) (bool, error) {
	f.events.add("fail_task")
	if f.failTaskErr != nil {
		return false, f.failTaskErr
	}
	if f.failTaskOK {
		f.failInputs = append(f.failInputs, input)
	}
	return f.failTaskOK, nil
}

func (f *fakeTransfers) FailDependents(prepareTaskID, errorCode, errorMessage string, now time.Time) (int64, error) {
	f.events.add("fail_dependents")
	if f.dependentsErr != nil {
		return 0, f.dependentsErr
	}
	return 1, nil
}

// fakeStore is an object store in a map. Every step is separately failable, `Copy`
// really copies so that the size read back at the formal key is a reading of bytes
// rather than of a constant the test supplied, and `statSize` is a function rather
// than a table because the formal key is a digest of bytes no test can name in
// advance.
type fakeStore struct {
	events *eventLog

	objects map[string][]byte

	putErr    error
	copyErr   error
	removeErr error

	statErr  map[string]error
	statSize func(key string, stored int64) int64

	statCalls []string
}

func newFakeStore(events *eventLog) *fakeStore {
	return &fakeStore{
		events:  events,
		objects: map[string][]byte{},
		statErr: map[string]error{},
	}
}

func (s *fakeStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (storage.Grant, error) {
	return storage.Grant{URL: "https://store.example.invalid/" + key, ExpiresAt: testNow.Add(ttl)}, nil
}

func (s *fakeStore) Stat(ctx context.Context, key string) (storage.ObjectInfo, error) {
	s.events.add("stat " + key)
	s.statCalls = append(s.statCalls, key)
	if err, found := s.statErr[key]; found {
		return storage.ObjectInfo{}, err
	}
	body, found := s.objects[key]
	if !found {
		return storage.ObjectInfo{}, fmt.Errorf("no object at %s", key)
	}
	size := int64(len(body))
	if s.statSize != nil {
		size = s.statSize(key, size)
	}
	return storage.ObjectInfo{Key: key, Size: size}, nil
}

func (s *fakeStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	s.events.add("put " + key)
	if s.putErr != nil {
		return s.putErr
	}
	// The declared size is checked against the bytes handed over. A fake that ignored
	// the argument would let a worker upload a file it had not measured and pass.
	content, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if int64(len(content)) != size {
		return fmt.Errorf("put declared %d bytes and passed %d", size, len(content))
	}
	s.objects[key] = content
	return nil
}

func (s *fakeStore) Copy(ctx context.Context, sourceKey, destinationKey string) error {
	s.events.add("copy " + sourceKey + " " + destinationKey)
	if s.copyErr != nil {
		return s.copyErr
	}
	body, found := s.objects[sourceKey]
	if !found {
		return fmt.Errorf("no object at %s", sourceKey)
	}
	s.objects[destinationKey] = append([]byte(nil), body...)
	return nil
}

func (s *fakeStore) Remove(ctx context.Context, key string) error {
	s.events.add("remove " + key)
	if s.removeErr != nil {
		return s.removeErr
	}
	delete(s.objects, key)
	return nil
}

// formalKeys are the keys this store holds that are not staging keys. The matrix's
// "wrote no formal object" is asserted against this, because the staging key is
// written on paths that later fail and must not count.
func (s *fakeStore) formalKeys() []string {
	var keys []string
	for key := range s.objects {
		if !strings.Contains(key, "/.staging/") {
			keys = append(keys, key)
		}
	}
	return keys
}

type fakeSources struct {
	events *eventLog

	body   io.ReadCloser
	source sourcefile.Source
	err    error

	addresses []string
	offsets   []int64
}

func (f *fakeSources) Open(ctx context.Context, address string, offset int64) (io.ReadCloser, sourcefile.Source, error) {
	f.events.add("open")
	f.addresses = append(f.addresses, address)
	f.offsets = append(f.offsets, offset)
	if f.err != nil {
		return nil, sourcefile.Source{}, f.err
	}
	return f.body, f.source, nil
}

type fakeDetails struct {
	events *eventLog

	address    string
	err        error
	platforms  []string
	contentIDs []int64
}

func (f *fakeDetails) Address(ctx context.Context, platform string, contentID int64) (string, error) {
	f.events.add("address")
	f.platforms = append(f.platforms, platform)
	f.contentIDs = append(f.contentIDs, contentID)
	if f.err != nil {
		return "", f.err
	}
	return f.address, nil
}

type fakeLog struct {
	mu      sync.Mutex
	entries []string
}

func (l *fakeLog) PreparationFinished(taskID string, materialID int64, outcome string, detail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, outcome+"|"+detail)
}

func (l *fakeLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...)
}

func (l *fakeLog) outcomes() []string {
	var outcomes []string
	for _, entry := range l.all() {
		outcomes = append(outcomes, strings.SplitN(entry, "|", 2)[0])
	}
	return outcomes
}

func (l *fakeLog) has(outcome string) bool {
	for _, recorded := range l.outcomes() {
		if recorded == outcome {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// The harness.
// ---------------------------------------------------------------------------

type harness struct {
	preparer  *Preparer
	events    *eventLog
	materials *fakeMaterials
	transfers *fakeTransfers
	store     *fakeStore
	sources   *fakeSources
	details   *fakeDetails
	log       *fakeLog
	body      []byte
	tempRoot  string
	ctx       context.Context
	cancel    context.CancelFunc
}

// harnessFor assembles a preparer whose every step succeeds, and hands the caller
// the fakes so a test can spoil exactly one of them.
func harnessFor(t *testing.T) *harness {
	t.Helper()
	events := &eventLog{}
	// The body is padded past several read blocks so that the heartbeat, which is
	// checked between blocks, has somewhere to fire. A one-block fixture would make
	// every lease assertion vacuous.
	body := mp4Fixture(4*copyBufferSize + 1)

	materials := &fakeMaterials{
		events: events,
		material: productionservice.PreparationSource{
			TeamID:     testTeamID,
			MaterialID: testMaterial,
			Platform:   douyinPlatform,
			ContentID:  testContentID,
		},
	}
	transfers := &fakeTransfers{
		events:      events,
		claimed:     transfermodel.Task{ID: testTaskID, TeamID: testTeamID, AssetID: testMaterial, Status: transfermodel.StatusRunning},
		claimFound:  true,
		heartbeatOK: true,
		completeOK:  true,
		failTaskOK:  true,
	}
	store := newFakeStore(events)
	sources := &fakeSources{
		events: events,
		body:   io.NopCloser(bytes.NewReader(body)),
		source: sourcefile.Source{RangeStart: 0, TotalBytes: int64(len(body)), ContentType: "video/mp4"},
	}
	details := &fakeDetails{events: events, address: testAddress}
	log := &fakeLog{}
	ctx, cancel := context.WithCancel(context.Background())

	tempRoot := t.TempDir()
	preparer := &Preparer{
		WorkerID:  testWorkerID,
		Lease:     30 * time.Second,
		TempDir:   tempRoot,
		Store:     store,
		Sources:   sources,
		Details:   details,
		Materials: materials,
		Transfers: transfers,
		Probe:     media.Probe,
		Now:       func() time.Time { return testNow },
		Log:       log,
	}
	t.Cleanup(func() {
		cancel()
		// Nothing is checked in before or left behind: a partial download is a file a
		// later attempt could mistake for a whole one.
		leftovers, err := filepath.Glob(filepath.Join(tempRoot, "wt-media-prepare-*"))
		if err != nil {
			t.Fatalf("glob the temporary root: %v", err)
		}
		if len(leftovers) != 0 {
			t.Errorf("the staged download survived the attempt: %v", leftovers)
		}
	})
	return &harness{
		preparer: preparer, events: events, materials: materials, transfers: transfers,
		store: store, sources: sources, details: details, log: log,
		body: body, tempRoot: tempRoot, ctx: ctx, cancel: cancel,
	}
}

func (h *harness) run(t *testing.T) {
	t.Helper()
	found, err := h.preparer.Prepare(h.ctx)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if !found {
		t.Fatalf("Prepare() claimed nothing, want the queued task")
	}
}

// assertNeverReady is the invariant every row of the matrix ends with: whatever
// failed, the material does not claim a source it does not have, and nothing was
// told the facts.
func (h *harness) assertNeverReady(t *testing.T) {
	t.Helper()
	if h.events.indexOf("ready") >= 0 {
		t.Errorf("the material was marked ready; events: %v", h.events.all())
	}
	if h.transfers.handoverCalled != 0 {
		t.Errorf("facts were handed over for bytes that were never verified")
	}
}

// assertNoFormalObject is the stronger assertion for the rows that fail before the
// object is committed: not merely "the material is not ready" but "nothing was left
// in the bucket". It is separate because two rows fail *after* the copy, and an
// assertion that a formal object does not exist would be false there rather than
// protective — the object does exist, at a key nothing points at, and the next
// attempt writes the same bytes to the same key.
func (h *harness) assertNoFormalObject(t *testing.T) {
	t.Helper()
	if keys := h.store.formalKeys(); len(keys) != 0 {
		t.Errorf("a formal object was written: %v", keys)
	}
}

func (h *harness) assertFailure(t *testing.T, code string) {
	t.Helper()
	if h.events.indexOf("fail_task") < 0 {
		t.Fatalf("the task was not failed; events: %v", h.events.all())
	}
	if len(h.transfers.failInputs) == 0 {
		t.Fatalf("FailTask recorded no input")
	}
	for _, input := range h.transfers.failInputs {
		if input.ErrorCode != code {
			continue
		}
		if input.Status != transfermodel.StatusFailed {
			t.Errorf("status = %q, want failed for code %q", input.Status, code)
		}
		if input.NodeID != testWorkerID {
			t.Errorf("node id = %q, want this worker", input.NodeID)
		}
		return
	}
	t.Errorf("error codes = %v, want %q", failCodes(h.transfers.failInputs), code)
}

func failCodes(inputs []transferrepo.FailureInput) []string {
	var codes []string
	for _, input := range inputs {
		codes = append(codes, input.ErrorCode)
	}
	return codes
}

// assertNoAddressInOutput is the boundary rule the CHG repeats in §4: a signed
// source URL is a credential, and the worker is one of the places it most easily
// escapes from.
func (h *harness) assertNoAddressInOutput(t *testing.T) {
	t.Helper()
	forbidden := []string{"DO-NOT-LOG-ME", "example.invalid"}
	for _, entry := range h.log.all() {
		for _, secret := range forbidden {
			if strings.Contains(entry, secret) {
				t.Errorf("a log line carries part of the signed address (%q): %q", secret, entry)
			}
		}
	}
	for _, failure := range h.materials.failed {
		for _, secret := range forbidden {
			if strings.Contains(failure, secret) {
				t.Errorf("the material's failure message carries part of the signed address (%q): %q", secret, failure)
			}
		}
	}
	for _, input := range h.transfers.failInputs {
		for _, secret := range forbidden {
			if strings.Contains(input.ErrorMessage, secret) {
				t.Errorf("the task's failure message carries part of the signed address (%q): %q", secret, input.ErrorMessage)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// The success path, first: it is what the matrix's invariants are measured
// against, and it is the only row that may write `ready`.
// ---------------------------------------------------------------------------

func TestPrepareWritesTheFactsOfTheBytesItHashed(t *testing.T) {
	h := harnessFor(t)
	h.run(t)

	wantFormal := fmt.Sprintf("materials/%d/%s.mp4", testMaterial, digestOf(h.body))
	wantStaging := fmt.Sprintf("materials/%d/.staging/%s.mp4", testMaterial, testTaskID)

	if len(h.materials.readyFacts) != 1 {
		t.Fatalf("MarkVideoReady called %d times, want 1", len(h.materials.readyFacts))
	}
	facts := h.materials.readyFacts[0]
	if facts.ObjectKey != wantFormal {
		t.Errorf("fact object key = %q, want %q", facts.ObjectKey, wantFormal)
	}
	if facts.SizeBytes != int64(len(h.body)) {
		t.Errorf("fact size = %d, want the %d bytes that were hashed", facts.SizeBytes, len(h.body))
	}
	if facts.SHA256 != digestOf(h.body) {
		t.Errorf("fact sha256 = %q, want the digest of the bytes that were hashed", facts.SHA256)
	}
	// The facts are the probe's reading of the file that was just written, decoded
	// back. A worker that wrote a constant, or the provider's idea of the file, fails
	// here.
	var probed media.Media
	if err := json.Unmarshal(facts.Media, &probed); err != nil {
		t.Fatalf("the media facts are not JSON: %v (%s)", err, facts.Media)
	}
	if probed.Container != "mp4" || probed.DurationMS != 5000 {
		t.Errorf("media facts = %+v, want the container and duration of the staged file", probed)
	}

	if h.transfers.handoverCalled != 1 {
		t.Fatalf("HandOverDependencies called %d times, want 1", h.transfers.handoverCalled)
	}
	if got := h.transfers.handoverFacts[0]; got.SourceObjectKey != wantFormal || got.TotalBytes != int64(len(h.body)) || got.ExpectedSHA256 != facts.SHA256 {
		t.Errorf("handed-over facts = %+v, want the same verified facts the material was given", got)
	}
	if len(h.transfers.completionInputs) != 1 {
		t.Fatalf("CompleteTask called %d times, want 1", len(h.transfers.completionInputs))
	}
	if got := h.transfers.completionInputs[0]; got.Bytes != int64(len(h.body)) || got.SHA256 != facts.SHA256 || got.NodeID != testWorkerID {
		t.Errorf("completion = %+v, want the measured size and digest under this worker's id", got)
	}
	if h.sources.offsets[0] != 0 {
		t.Errorf("the source was opened at offset %d, want the whole file from zero", h.sources.offsets[0])
	}
	if _, stillStaged := h.store.objects[wantStaging]; stillStaged {
		t.Errorf("the staging object was left behind at %s", wantStaging)
	}
	if !h.log.has("success") {
		t.Errorf("outcomes = %v, want a success", h.log.outcomes())
	}
	if body, found := h.store.objects[wantFormal]; !found || !bytes.Equal(body, h.body) {
		t.Errorf("the formal object does not hold the bytes that were downloaded")
	}
	h.assertNoAddressInOutput(t)
}

// The log is how an operator finds out what happened, not part of what happens. A
// preparation that only settled its rows when someone was listening would lose them
// on the one worker nobody is watching, and `record` is called on every path out of
// this worker — including the ones taken while it is already failing.
func TestPrepareSettlesItsRowsWithoutALogSink(t *testing.T) {
	t.Run("a preparation that succeeds", func(t *testing.T) {
		h := harnessFor(t)
		h.preparer.Log = nil

		h.run(t)

		if len(h.materials.readyFacts) != 1 {
			t.Errorf("MarkVideoReady called %d times, want 1", len(h.materials.readyFacts))
		}
		if len(h.transfers.completionInputs) != 1 {
			t.Errorf("CompleteTask called %d times, want 1", len(h.transfers.completionInputs))
		}
	})
	t.Run("a preparation that fails", func(t *testing.T) {
		h := harnessFor(t)
		h.preparer.Log = nil
		h.materials.sourceErr = errors.New("material 42 names no platform source to re-resolve")

		h.run(t)

		if len(h.transfers.failInputs) != 1 {
			t.Errorf("FailTask called %d times, want 1", len(h.transfers.failInputs))
		}
		if len(h.materials.failed) != 1 {
			t.Errorf("MarkVideoFailed called %d times, want 1", len(h.materials.failed))
		}
	})
}

// The order is the mechanism, not an implementation detail: the facts are written
// only after the object is verified at its formal key, and the waiters are released
// before the task is closed. A crash between any two of these is recoverable in
// exactly one order.
func TestPrepareVerifiesTheFormalKeyBeforeItWritesAnythingAboutIt(t *testing.T) {
	h := harnessFor(t)
	h.run(t)

	staging := "materials/42/.staging/task-1.mp4"
	formal := "materials/42/" + digestOf(h.body) + ".mp4"
	want := []string{
		"claim",
		"resolve",
		"address",
		"open",
		"put " + staging,
		"stat " + staging,
		"copy " + staging + " " + formal,
		"stat " + formal,
		"remove " + staging,
		"ready",
		"handover",
		"complete",
	}
	got := h.events.all()
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("event %d = %q, want %q (all: %v)", index, got[index], want[index], got)
		}
	}
}

// The probe runs on the bytes on disk, before the store is touched at all. That is
// a deliberate deviation from the plan's stated order, and this pins it: a container
// that cannot be read is not worth two uploads and an object left at a key nothing
// refers to.
func TestPrepareRefusesAFileTheProbeWillNotReadBeforeItUploadsAnything(t *testing.T) {
	h := harnessFor(t)
	h.sources.body = io.NopCloser(strings.NewReader("<html>not a video</html>"))
	h.sources.source = sourcefile.Source{TotalBytes: int64(len("<html>not a video</html>"))}
	h.run(t)

	h.assertNeverReady(t)
	if h.events.indexOf("put materials/42/.staging/task-1.mp4") >= 0 {
		t.Errorf("the file was uploaded before the probe refused it: %v", h.events.all())
	}
	h.assertFailure(t, codeProbeRefused)
	if len(h.materials.failed) != 1 || !strings.Contains(h.materials.failed[0], "not an ISO base media file") {
		t.Errorf("material failure message = %v, want the probe's own reason", h.materials.failed)
	}
}

// ---------------------------------------------------------------------------
// The matrix: one row per way the preparation can go wrong.
// ---------------------------------------------------------------------------

func TestPrepareFailureMatrix(t *testing.T) {
	const shortBody = "short"

	cases := []struct {
		name string
		// spoil is applied to an otherwise-working harness.
		spoil func(*harness)
		// code is the error code the task must carry, and fragment is the text a
		// failure message must contain so that the code and the message agree.
		code     string
		fragment string
		// committed marks the rows that fail after the copy, where a formal object
		// legitimately exists at a key nothing points at. Every other row additionally
		// asserts that the bucket was left untouched.
		committed bool
	}{
		{
			name: "the material names no platform source",
			spoil: func(h *harness) {
				h.materials.sourceErr = errors.New("material 42 names no platform source to re-resolve")
			},
			code: codeSourceUnresolved, fragment: "names no platform source",
		},
		{
			name: "the detail endpoint could not be reached",
			spoil: func(h *harness) {
				h.details.err = errors.New("douyin detail request failed with status 500")
			},
			code: codeDetailFailed, fragment: "status 500",
		},
		{
			name: "the payload carries no playable address",
			spoil: func(h *harness) {
				h.details.err = fmt.Errorf("resolving the address: %w", douyin.ErrNoMediaAddress)
			},
			code: codeAddressMissing, fragment: "no playable address",
		},
		{
			name: "the detail response carried no item",
			spoil: func(h *harness) {
				h.details.err = fmt.Errorf("reading the detail: %w", douyin.ErrNoDetailItem)
			},
			code: codeAddressMissing, fragment: "no video item",
		},
		{
			name: "the source was not found",
			spoil: func(h *harness) {
				h.sources.err = errors.New("source request failed with status 404")
			},
			code: codeFetchFailed, fragment: "status 404",
		},
		{
			name: "the source answered with no bytes",
			spoil: func(h *harness) {
				h.sources.body = io.NopCloser(bytes.NewReader(nil))
				h.sources.source = sourcefile.Source{TotalBytes: 0}
			},
			code: codeFetchFailed, fragment: "no bytes",
		},
		{
			name: "the body was shorter than the server declared",
			spoil: func(h *harness) {
				h.sources.body = io.NopCloser(strings.NewReader(shortBody))
				h.sources.source = sourcefile.Source{TotalBytes: int64(len(h.body))}
			},
			code: codeFetchFailed, fragment: "declared",
		},
		{
			name: "a chunked answer that declared no length is accepted on the size received",
			spoil: func(h *harness) {
				h.sources.source = sourcefile.Source{TotalBytes: -1}
			},
			code: "", fragment: "",
		},
		{
			name: "the body did not start where it was asked to",
			spoil: func(h *harness) {
				h.sources.source = sourcefile.Source{RangeStart: 1024, TotalBytes: int64(len(h.body))}
			},
			code: codeFetchFailed, fragment: "offset 1024",
		},
		{
			name: "the body broke midway",
			spoil: func(h *harness) {
				h.sources.body = io.NopCloser(&breakingReader{body: h.body, failAfter: 1})
				h.sources.source = sourcefile.Source{TotalBytes: int64(len(h.body))}
			},
			code: codeFetchFailed, fragment: "connection reset by peer",
		},
		{
			name: "the staged object could not be written",
			spoil: func(h *harness) {
				h.store.putErr = errors.New("the object store refused the request")
			},
			code: codeStagingFailed, fragment: "object store refused",
		},
		{
			name: "the staged object did not read back as uploaded",
			spoil: func(h *harness) {
				h.store.statSize = func(key string, stored int64) int64 {
					if strings.Contains(key, "/.staging/") {
						return stored + 1
					}
					return stored
				}
			},
			code: codeStagingVerify, fragment: "bytes, but",
		},
		{
			name: "the staged object could not be read back at all",
			spoil: func(h *harness) {
				h.store.statErr["materials/42/.staging/task-1.mp4"] = errors.New("the object store is unreachable")
			},
			code: codeStagingVerify, fragment: "unreachable",
		},
		{
			name: "the copy to the formal key failed",
			spoil: func(h *harness) {
				h.store.copyErr = errors.New("the object store refused the copy")
			},
			code: codeCopyFailed, fragment: "refused the copy",
		},
		{
			// The formal key is a digest of bytes, so this row names no key: the store
			// is told to over-report whatever it is asked about at a formal key.
			name: "the formal object did not read back as uploaded",
			spoil: func(h *harness) {
				h.store.statSize = func(key string, stored int64) int64 {
					if strings.Contains(key, "/.staging/") {
						return stored
					}
					return stored + 1
				}
			},
			code: codeCommitVerify, fragment: "bytes, but", committed: true,
		},
		{
			name: "the temporary directory could not be created",
			spoil: func(h *harness) {
				// A path below a regular file can never be a directory, which is the same
				// condition as a full or unwritable volume without needing one.
				blocker := filepath.Join(h.tempRoot, "not-a-directory")
				if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
					t.Fatalf("write the blocking file: %v", err)
				}
				h.preparer.TempDir = filepath.Join(blocker, "below")
			},
			code: codeFetchFailed, fragment: "temporary download directory",
		},
		{
			name: "the context was cancelled while the body was arriving",
			spoil: func(h *harness) {
				h.sources.body = io.NopCloser(&cancellingReader{body: h.body, cancel: h.cancel})
				h.sources.source = sourcefile.Source{TotalBytes: int64(len(h.body))}
			},
			code: codeFetchFailed, fragment: "context canceled",
		},
		{
			name: "the lease could not be renewed",
			spoil: func(h *harness) {
				h.preparer.Now = steppingClock(10 * time.Second)
				h.transfers.heartbeatErr = errors.New("the database is unreachable")
			},
			code: codeFetchFailed, fragment: "renew the transfer lease",
		},
		{
			name: "the material write failed after the object was verified",
			spoil: func(h *harness) {
				h.materials.readyErr = errors.New("the materials table is unreachable")
			},
			code: codeCommitVerify, fragment: "materials table is unreachable", committed: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := harnessFor(t)
			testCase.spoil(h)
			h.run(t)

			if testCase.code == "" {
				// The one row that is not a failure: an unknown length is a question the
				// worker does not need answered, and the size that arrived is the answer it
				// stores. Asserting the success keeps this row from silently becoming a
				// duplicate of the truncation row above it.
				if len(h.materials.readyFacts) != 1 || h.materials.readyFacts[0].SizeBytes != int64(len(h.body)) {
					t.Fatalf("the undeclared-length answer was refused instead of measured: %v", h.log.outcomes())
				}
				return
			}

			h.assertNeverReady(t)
			if !testCase.committed {
				h.assertNoFormalObject(t)
			}
			h.assertFailure(t, testCase.code)
			if len(h.materials.failed) != 1 {
				t.Fatalf("material failure messages = %v, want exactly one", h.materials.failed)
			}
			if !strings.Contains(h.materials.failed[0], testCase.fragment) {
				t.Errorf("material failure message = %q, want it to mention %q", h.materials.failed[0], testCase.fragment)
			}
			h.assertNoAddressInOutput(t)
		})
	}
}

// ---------------------------------------------------------------------------
// The lease, which is the whole of this worker's liveness handling.
// ---------------------------------------------------------------------------

// A body that arrives in blocks renews the lease as it goes. The alternative — a
// timer — would keep the lease alive while nothing was arriving, so the row would
// read `running` for as long as the worker lived and the task would never be
// retried.
func TestPrepareRenewsTheLeaseWhileTheBodyArrives(t *testing.T) {
	h := harnessFor(t)
	// A clock that steps by a third of the lease on every reading makes the renewal
	// fire a fixed number of times, so the assertion is about the mechanism rather
	// than about how fast the machine is.
	h.preparer.Now = steppingClock(10 * time.Second)
	h.run(t)

	if h.transfers.heartbeats == 0 {
		t.Fatalf("the lease was never renewed over %d bytes", len(h.body))
	}
	if !h.log.has("success") {
		t.Errorf("outcomes = %v, want a success", h.log.outcomes())
	}
}

// A renewal that is refused is the cancellation and lost-lease signal. Mid-download
// it means nothing was ever handed over, so the material is not written about at all
// — and the waiters are released, because a `pending` download pointing at a
// terminal task is unclaimable forever.
func TestPrepareStopsWhenTheLeaseIsNoLongerHeld(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		current transfermodel.Task
		status  transfermodel.Status
		code    string
	}{
		{
			name:    "the lease lapsed and a retry took the task",
			current: transfermodel.Task{ID: testTaskID},
			status:  transfermodel.StatusFailed,
			code:    codeLeaseLost,
		},
		{
			name:    "the user cancelled while the bytes were arriving",
			current: transfermodel.Task{ID: testTaskID, CancelRequestedAt: &testNow},
			status:  transfermodel.StatusCancelled,
			code:    codeCancelled,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := harnessFor(t)
			h.transfers.heartbeatOK = false
			h.transfers.current = testCase.current
			h.preparer.Now = steppingClock(10 * time.Second)
			h.run(t)

			h.assertNeverReady(t)
			if h.events.indexOf("get") < 0 {
				t.Errorf("the task's row was never read; events: %v", h.events.all())
			}
			if h.events.indexOf("fail_dependents") < 0 {
				t.Errorf("the waiting downloads were not released; events: %v", h.events.all())
			}
			if len(h.transfers.failInputs) != 1 {
				t.Fatalf("terminal writes = %v, want exactly one", failCodes(h.transfers.failInputs))
			}
			if got := h.transfers.failInputs[0]; got.Status != testCase.status || got.ErrorCode != testCase.code {
				t.Errorf("terminal write = (%q, %q), want (%q, %q)", got.Status, got.ErrorCode, testCase.status, testCase.code)
			}
			// Nothing was verified, so the material was told nothing at all.
			if len(h.materials.failed) != 0 || len(h.materials.readyFacts) != 0 {
				t.Errorf("the material was written about, but no bytes were verified: failed=%v ready=%d", h.materials.failed, len(h.materials.readyFacts))
			}
		})
	}
}

// The cancellation can also arrive after the bytes are verified and committed. The
// material is then genuinely ready and the waiters have their facts, so the only
// thing left is for the task row to say what actually happened to it.
func TestPrepareRecordsACancellationThatArrivedAfterTheSourceWasVerified(t *testing.T) {
	h := harnessFor(t)
	h.transfers.completeOK = false
	h.transfers.current = transfermodel.Task{ID: testTaskID, CancelRequestedAt: &testNow}
	h.run(t)

	if len(h.materials.readyFacts) != 1 {
		t.Fatalf("the verified source was discarded: MarkVideoReady called %d times", len(h.materials.readyFacts))
	}
	if len(h.transfers.failInputs) != 1 {
		t.Fatalf("terminal writes = %v, want exactly one", failCodes(h.transfers.failInputs))
	}
	got := h.transfers.failInputs[0]
	if got.Status != transfermodel.StatusCancelled || got.ErrorCode != codeCancelled {
		t.Errorf("terminal write = (%q, %q), want (cancelled, %q)", got.Status, got.ErrorCode, codeCancelled)
	}
	if !h.log.has(string(transfermodel.StatusCancelled)) {
		t.Errorf("outcomes = %v, want the cancellation recorded", h.log.outcomes())
	}
}

// A failure to write the completion is not a failure of the preparation.
// Everything a user can see is already correct, so recording a terminal failure
// would make a successful preparation read as a failed one.
func TestPrepareLeavesTheTaskOpenWhenOnlyTheCompletionWriteFailed(t *testing.T) {
	h := harnessFor(t)
	h.transfers.completeErr = errors.New("the database is unreachable")
	h.run(t)

	if len(h.materials.readyFacts) != 1 {
		t.Fatalf("MarkVideoReady called %d times, want 1", len(h.materials.readyFacts))
	}
	if h.events.indexOf("fail_task") >= 0 {
		t.Errorf("a successful preparation was recorded as a failure: %v", h.events.all())
	}
	if !h.log.has("completion write failed") {
		t.Errorf("outcomes = %v, want the failed completion write recorded", h.log.outcomes())
	}
}

// A hand-over that failed is not a source that failed: the material is ready and
// says so. The waiters are told anyway, because nothing will ever hand them facts.
func TestPrepareDoesNotBlameTheSourceForAFailedHandOver(t *testing.T) {
	h := harnessFor(t)
	h.transfers.handoverErr = errors.New("the database is unreachable")
	h.run(t)

	if len(h.materials.readyFacts) != 1 {
		t.Fatalf("MarkVideoReady called %d times, want 1", len(h.materials.readyFacts))
	}
	if len(h.materials.failed) != 0 {
		t.Errorf("the material was told its source failed, but it was verified: %v", h.materials.failed)
	}
	h.assertFailure(t, codeHandoverFailed)
	if h.events.indexOf("fail_dependents") < 0 {
		t.Errorf("the waiting downloads were left pointing at a failed task; events: %v", h.events.all())
	}
}

// ---------------------------------------------------------------------------
// The claim, and the shapes a caller sees when there is nothing to do.
// ---------------------------------------------------------------------------

func TestPrepareClaimsOneTaskAndRunsIt(t *testing.T) {
	h := harnessFor(t)
	h.run(t)
	if h.events.indexOf("claim") != 0 {
		t.Errorf("the first thing that happened was %v, want a claim", h.events.all())
	}
	if h.transfers.claimFound && !strings.HasPrefix(h.preparer.WorkerID, "cloud-worker:") {
		t.Errorf("worker id = %q, want the prefix the claim predicate is read with", h.preparer.WorkerID)
	}
}

func TestPrepareReportsAnEmptyQueue(t *testing.T) {
	h := harnessFor(t)
	h.transfers.claimFound = false
	found, err := h.preparer.Prepare(h.ctx)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if found {
		t.Errorf("Prepare() reported a task, want none")
	}
	if len(h.events.all()) != 1 {
		t.Errorf("events = %v, want only the claim", h.events.all())
	}
}

func TestPrepareReportsAFailedClaim(t *testing.T) {
	h := harnessFor(t)
	h.transfers.claimErr = errors.New("the database is unreachable")
	found, err := h.preparer.Prepare(h.ctx)
	if err == nil {
		t.Fatalf("Prepare() error = nil, want the claim's failure")
	}
	if found {
		t.Errorf("Prepare() reported a task alongside a failed claim")
	}
	if !strings.Contains(err.Error(), "claim a cloud preparation") {
		t.Errorf("error = %v, want it to name the step", err)
	}
}

// A worker with nowhere to put the bytes must not claim one: it would spend an
// attempt and mark a material failed for a condition only an operator can fix.
func TestRunMaterialPrepareWorkerDoesNothingWithoutObjectStorage(t *testing.T) {
	if err := storage.Close(); err != nil {
		t.Fatalf("storage.Close() error = %v", err)
	}
	if storage.Configured() {
		t.Fatalf("storage reports itself configured after being closed")
	}
	// It must return without reaching the store at all. `storage.Get()` panics when
	// nothing was published, so a guard that was deleted or reordered fails loudly
	// here rather than silently claiming a task it cannot serve.
	if err := RunMaterialPrepareWorker(context.Background()); err != nil {
		t.Fatalf("RunMaterialPrepareWorker() error = %v", err)
	}
}

// A published-but-credential-less store is the state every tree in this repository
// ships in, and it is the same answer as no store at all.
func TestRunMaterialPrepareWorkerDoesNothingWithACredentiallessStore(t *testing.T) {
	if err := storage.Close(); err != nil {
		t.Fatalf("storage.Close() error = %v", err)
	}
	if err := storage.Initialize(config.ObjectStorageConfig{}, config.ObjectStorageCredentialConfig{}); err != nil {
		t.Fatalf("storage.Initialize() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	if storage.Configured() {
		t.Fatalf("storage reports itself configured without a credential")
	}
	if err := RunMaterialPrepareWorker(context.Background()); err != nil {
		t.Fatalf("RunMaterialPrepareWorker() error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// The units the worker owns that are not part of the flow.
// ---------------------------------------------------------------------------

// The message goes into two places with different bounds: `error_message
// VARCHAR(500)`, which MySQL counts in characters, and a repository guard that
// refuses more than 500 *bytes*. The stricter bound is bytes, and a cut through the
// middle of a character writes a broken one into a row people read.
func TestFailureTextStaysWithinTheBytesTheColumnAndTheGuardAccept(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		message string
	}{
		{"a short ascii message", "the source refused"},
		{"exactly at the bound", strings.Repeat("a", failureTextLimit)},
		{"one byte over", strings.Repeat("a", failureTextLimit+1)},
		{"far over", strings.Repeat("a", failureTextLimit*4)},
		{"far over, in three-byte characters", strings.Repeat("素", failureTextLimit)},
		{"far over, mixing widths", strings.Repeat("素材a", failureTextLimit)},
		{"surrounded by whitespace", "  " + strings.Repeat("a", failureTextLimit) + "\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := failureText(testCase.message)
			if len(got) > failureTextLimit {
				t.Fatalf("len(failureText(...)) = %d, want at most %d", len(got), failureTextLimit)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("the cut produced an invalid string: %q", got)
			}
			if strings.TrimSpace(got) == "" {
				t.Fatalf("failureText(...) = %q, want the message kept", got)
			}
			// A message that already fits is passed through untouched. The bound is a
			// limit on the row, not a rewrite of every message that goes into it: cutting
			// one that fits would drop its last character and append an ellipsis, so a
			// message that is complete would read as one that continues.
			trimmed := strings.TrimSpace(testCase.message)
			if len(trimmed) <= failureTextLimit {
				if got != trimmed {
					t.Fatalf("failureText() = %q, want it unchanged at %q", got, trimmed)
				}
				return
			}
			// One that does not fit keeps its beginning and says it was cut.
			if !strings.HasSuffix(got, "…") {
				t.Fatalf("failureText() = %q, want a message that says it was cut", got)
			}
			if !strings.HasPrefix(trimmed, strings.TrimSuffix(got, "…")) {
				t.Fatalf("failureText() = %q, want the message's own beginning", got)
			}
		})
	}
}

// The codes are what the downloads list shows a person, so the step that refused
// has to be distinguishable from the step underneath it.
func TestCommitCodeNamesTheStepThatRefused(t *testing.T) {
	for _, testCase := range []struct {
		name string
		err  error
		want string
	}{
		{"the upload", fmt.Errorf("%w: %v", errStagingFailed, errors.New("x")), codeStagingFailed},
		{"the staged read-back", fmt.Errorf("%w: %v", errStagingVerify, errors.New("x")), codeStagingVerify},
		{"the copy", fmt.Errorf("%w: %v", errCopyFailed, errors.New("x")), codeCopyFailed},
		{"the formal read-back", fmt.Errorf("%w: %v", errCommitVerify, errors.New("x")), codeCommitVerify},
		// A key that could not be built never reached a step, and is reported as the
		// staged read-back rather than as a step that did not happen.
		{"a key that could not be built", errors.New("media extension must be 1 to 8 characters"), codeStagingVerify},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := commitCode(testCase.err); got != testCase.want {
				t.Errorf("commitCode() = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A platform this build cannot read is refused rather than guessed at, and the
// refusal happens before any client is reached.
func TestPlatformMediaAddressesRefusesAPlatformItCannotRead(t *testing.T) {
	for _, platform := range []string{"", "  ", "kuaishou", "Douyin"} {
		t.Run(platform, func(t *testing.T) {
			address, err := platformMediaAddresses{}.Address(context.Background(), platform, testContentID)
			if err == nil {
				t.Fatalf("Address(%q) = %q, nil, want a refusal", platform, address)
			}
			if address != "" {
				t.Errorf("address = %q, want none alongside the refusal", address)
			}
		})
	}
}

func TestCloudWorkerIDNamesThisProcessDistinctly(t *testing.T) {
	first, second := cloudWorkerID(), cloudWorkerID()
	if !strings.HasPrefix(first, "cloud-worker:") {
		t.Errorf("id = %q, want the cloud-worker prefix the claim predicate is read with", first)
	}
	if first == second {
		t.Errorf("two ids in one process are equal (%q); a restart would look like the same worker", first)
	}
	if strings.ContainsAny(first, " \t\n") {
		t.Errorf("id = %q, want no whitespace in a column value", first)
	}
}

// ---------------------------------------------------------------------------
// Fixtures and small helpers.
// ---------------------------------------------------------------------------

// mp4Fixture builds a file `media.Probe` accepts, assembled here rather than checked
// in: a committed fixture is a binary nothing reviews, and this one's every byte is
// a statement about the box layout the probe reads.
//
// `ftyp` with a major brand, then `moov` holding a version-0 `mvhd` whose timescale
// and duration give a whole number of milliseconds, then a `free` box of padding so
// that the body crosses several read blocks. The probe stops at `moov`, so the
// padding is never parsed and exists only to make the download multi-block.
func mp4Fixture(padding int) []byte {
	mvhd := make([]byte, 20)
	// mvhd[0] is the version; zero selects the 32-bit layout the probe reads.
	binary.BigEndian.PutUint32(mvhd[12:], 1000)
	binary.BigEndian.PutUint32(mvhd[16:], 5000)

	ftyp := append([]byte("isom"), 0, 0, 2, 0)
	ftyp = append(ftyp, []byte("isomiso2")...)

	out := append(mp4Box("ftyp", ftyp), mp4Box("moov", mp4Box("mvhd", mvhd))...)
	return append(out, mp4Box("free", make([]byte, padding))...)
}

func mp4Box(kind string, payload []byte) []byte {
	out := make([]byte, 0, 8+len(payload))
	out = binary.BigEndian.AppendUint32(out, uint32(8+len(payload)))
	out = append(out, kind...)
	return append(out, payload...)
}

func digestOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// breakingReader serves the body and then fails, which is a connection dropped
// partway through rather than a short answer.
type breakingReader struct {
	body      []byte
	failAfter int
	reads     int
}

func (r *breakingReader) Read(into []byte) (int, error) {
	r.reads++
	if r.reads > r.failAfter {
		return 0, errors.New("connection reset by peer")
	}
	if len(r.body) == 0 {
		return 0, io.EOF
	}
	count := copy(into, r.body)
	r.body = r.body[count:]
	return count, nil
}

// cancellingReader cancels the worker's context once the first block has been
// served, so the abort lands mid-body rather than before the download started.
type cancellingReader struct {
	body   []byte
	cancel context.CancelFunc
	served bool
}

func (r *cancellingReader) Read(into []byte) (int, error) {
	if r.served {
		return 0, errors.New("the download read again after the context was cancelled")
	}
	r.served = true
	count := copy(into, r.body)
	r.body = r.body[count:]
	r.cancel()
	return count, nil
}

// steppingClock advances by a fixed step on every reading, which makes the renewal
// interval fire a fixed number of times over a fixed number of blocks. A real clock
// would make the assertion depend on how fast the machine is.
func steppingClock(step time.Duration) func() time.Time {
	var mu sync.Mutex
	now := testNow
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		now = now.Add(step)
		return now
	}
}
