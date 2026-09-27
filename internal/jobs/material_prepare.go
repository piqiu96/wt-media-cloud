package jobs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/sourcefile"
	"github.com/wt-media/wt-media-cloud/internal/infra/logger"
	"github.com/wt-media/wt-media-cloud/internal/infra/media"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	transfermodel "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	transferrepo "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/repository"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	producerepo "github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	productionservice "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

// The Cloud half of a material preparation: claim the task a click queued, fetch
// the source the platform named, verify it, commit it under the key its own bytes
// derive, and only then write the facts onto the material and release the
// downloads waiting on it.
//
// Everything before the last two steps leaves the material exactly where it was.
// The one thing that must never happen is a `ready` projection written for bytes
// that did not pass this path, so the writes that produce one come last and the
// facts they carry come from the file that was just hashed, never from anything
// the network said about it.

// A preparation's error codes. They are free strings in the frozen contract
// (`error_code` is bounded, not enumerated), so each one is chosen for the person
// reading the downloads list, and each names a different thing to do about it.
const (
	codeSourceUnresolved = "material_source_unresolved"
	codeDetailFailed     = "source_detail_failed"
	codeAddressMissing   = "source_address_missing"
	codeFetchFailed      = "source_fetch_failed"
	codeProbeRefused     = "source_not_readable"
	codeStagingFailed    = "upload_staging_failed"
	codeStagingVerify    = "upload_staging_unverified"
	codeCopyFailed       = "commit_copy_failed"
	codeCommitVerify     = "commit_unverified"
	codeHandoverFailed   = "dependency_handover_failed"
	codeLeaseLost        = "lease_lost"
	// codeCancelled is the code the cancellation reconciler writes, so a task
	// stopped by the user reads the same whether the executor noticed the stop or
	// the reconciliation did.
	codeCancelled = "cancelled_by_user"
)

// failureTextLimit is how long a failure message may be, and it is two bounds at
// once: `file_transfer_tasks.error_message` is `VARCHAR(500)`, which MySQL counts
// in characters, and both `FailTask` and `FailDependents` refuse a message longer
// than 500 *bytes* before the statement runs. The stricter bound is bytes, and a
// message that fits in bytes fits in characters too.
const failureTextLimit = 500

// errLeaseLost is how a step reports that this worker no longer owns its task.
// It is not a failure of the work — the bytes may be perfectly good — so it is
// answered differently: the task's own row says which terminal state it is in.
var errLeaseLost = errors.New("the transfer lease is no longer held by this worker")

// The commit's steps, as sentinels. Each is wrapped around the detail the store
// returned, so the code written to the row comes from `errors.Is` rather than from
// reading the message back out of a string.
var (
	errStagingFailed = errors.New("the staged object could not be written")
	errStagingVerify = errors.New("the staged object did not read back as uploaded")
	errCopyFailed    = errors.New("the staged object could not be copied to its final key")
	errCommitVerify  = errors.New("the final object did not read back as uploaded")
)

// SourceOpener opens a source address. It is `sourcefile`'s one method, narrowed
// so that a test answers it without a server.
type SourceOpener interface {
	Open(ctx context.Context, address string, offset int64) (io.ReadCloser, sourcefile.Source, error)
}

// MediaAddresses resolves the address a video's bytes are fetched from.
//
// The platform's payload never leaves the implementation: a worker that handled
// the provider's response would be a second place that knows which field holds the
// source, and the first place is one function with one test. An address is a
// short-lived signed URL, so it is also the value most easily lost to a log line,
// and keeping the payload behind this seam is what keeps this file from holding
// one at all.
type MediaAddresses interface {
	Address(ctx context.Context, platform string, contentID int64) (string, error)
}

// MaterialProjection is the whole of this worker's dependency on the production
// module: it reads the identity a preparation resolves from and writes the
// readiness projection, and nothing else. A worker that could read materials
// generally would eventually decide something else about them.
type MaterialProjection interface {
	ResolvePreparationSource(teamID identity.TeamID, materialID int64) (productionservice.PreparationSource, error)
	MarkVideoReady(teamID identity.TeamID, materialID int64, facts producerepo.VideoFacts) error
	MarkVideoFailed(teamID identity.TeamID, materialID int64, message string) error
	// MarkVideoNotPrepared is the cancellation's counterpart to `MarkVideoFailed`,
	// and it is not a failure write: the material goes back to saying it is not
	// downloaded, not to saying a preparation failed.
	MarkVideoNotPrepared(teamID identity.TeamID, materialID int64) error
}

// TransferQueue is the whole of this worker's dependency on the transfer module:
// the Cloud-executor half of its repository, whose functions were written for this
// caller — nothing else claims a task without a node credential, and nothing else
// writes the terminal `cancelled` row a cancel request leaves behind.
//
// The order the methods must be called in is the part no signature enforces, and
// it is stated here because it is a correctness rule rather than a preference:
// `HandOverDependencies` before `CompleteTask`. A worker that dies between them
// leaves a task that is retried, and the second attempt hands over facts that are
// already handed over and then completes. The reverse order leaves downloads
// waiting forever on a pointer to a task that has already finished.
type TransferQueue interface {
	ClaimCloudTask(workerID string, now time.Time, lease time.Duration) (transfermodel.Task, bool, error)
	GetTask(taskID string) (transfermodel.Task, error)
	HeartbeatTask(taskID, nodeID string, now time.Time, lease time.Duration) (bool, error)
	HandOverDependencies(prepareTaskID string, facts transferrepo.DependencyFacts, now time.Time) (int64, error)
	CompleteTask(input transferrepo.CompletionInput, now time.Time) (bool, error)
	FailTask(input transferrepo.FailureInput, now time.Time) (bool, error)
	FailDependents(prepareTaskID, errorCode, errorMessage string, now time.Time) (int64, error)
}

// Preparer runs one preparation at a time. Its fields are the seams a failure
// matrix needs: every step that can fail in production is replaceable here.
type Preparer struct {
	WorkerID string
	// Lease is how long a claim lasts and how often the download renews it. Empty
	// means `filetransfer`'s own default, so that the two executors cannot drift
	// into different retry timing.
	Lease time.Duration
	// TempDir is where the download is staged before it is uploaded. It is
	// deliberately outside the checkout: a test that passed the repository root
	// would write a video into the working tree, and `go test` would leave it there.
	// Empty means the operating system's temporary directory.
	TempDir   string
	Store     storage.Store
	Sources   SourceOpener
	Details   MediaAddresses
	Materials MaterialProjection
	Transfers TransferQueue
	// Probe reads what the downloaded bytes declare about themselves, and is
	// `media.Probe` in production. It is a field because "the file was written and
	// the probe refused it" is one of the matrix's rows.
	Probe func(io.ReaderAt, int64) (media.Media, error)
	Now   func() time.Time
	Log   PrepareLog
}

// PrepareLog is the one thing this worker says out loud. It is an interface rather
// than a logger so that a test can assert what was recorded without a log sink,
// and it takes no address: those URLs are credentials, and a worker is one of the
// places they most easily escape from.
type PrepareLog interface {
	// PreparationFinished records the outcome of one task. `outcome` is the word an
	// operator greps for and `detail` is why.
	PreparationFinished(taskID string, materialID int64, outcome string, detail string)
}

// Prepare claims at most one task and runs it. The boolean reports whether there
// was one.
func (p *Preparer) Prepare(ctx context.Context) (bool, error) {
	task, found, err := p.Transfers.ClaimCloudTask(p.WorkerID, p.now(), p.lease())
	if err != nil {
		return false, fmt.Errorf("claim a cloud preparation: %w", err)
	}
	if !found {
		return false, nil
	}
	p.run(ctx, task)
	return true, nil
}

// run executes one claimed task and records its outcome.
//
// Nothing here returns an error to the caller. The task's own row is where a
// preparation's outcome is written down, and a step that failed has already put it
// there; returning it as well would only stop the worker's loop over a task that
// has already been dealt with.
func (p *Preparer) run(ctx context.Context, task transfermodel.Task) {
	// failBeforeReady is the outcome of a preparation that produced no verified
	// source. All three places that could believe otherwise are told: the material
	// the user sees, the downloads waiting on this task, and the task itself.
	failBeforeReady := func(code, message string) {
		if err := p.Materials.MarkVideoFailed(task.TeamID, task.AssetID, message); err != nil {
			// The task and its waiters are still settled below. A projection that
			// could not be written leaves the material reading as "being prepared",
			// which the downloads list still shows as a stalled task — the failure is
			// recorded, not silent.
			p.record(task, "material write failed", err.Error())
		}
		p.failTaskAndWaiters(task, code, message)
	}

	source, err := p.Materials.ResolvePreparationSource(task.TeamID, task.AssetID)
	if err != nil {
		failBeforeReady(codeSourceUnresolved, err.Error())
		return
	}
	address, err := p.Details.Address(ctx, source.Platform, source.ContentID)
	if err != nil {
		// A payload that carries no address is a different answer from an endpoint
		// that could not be reached, and the two lead to different actions: one is a
		// material the provider has nothing to serve for, the other is worth retrying.
		code := codeDetailFailed
		if errors.Is(err, douyin.ErrNoMediaAddress) || errors.Is(err, douyin.ErrNoDetailItem) {
			code = codeAddressMissing
		}
		failBeforeReady(code, err.Error())
		return
	}

	download, err := p.download(ctx, task, address)
	if err != nil {
		if errors.Is(err, errLeaseLost) {
			p.settleLostLease(task, err)
			return
		}
		failBeforeReady(codeFetchFailed, err.Error())
		return
	}
	// The bytes never outlive the attempt, on any path from here: a partial file left
	// behind is a file a later attempt could mistake for a whole one.
	defer os.RemoveAll(download.dir)

	probed, err := p.probeFile(download.path, download.size)
	if err != nil {
		// The bytes are what the source served and the probe would not read them, so
		// nothing about them is verified. The probe's own message is the detail.
		failBeforeReady(codeProbeRefused, err.Error())
		return
	}

	facts, err := p.commit(ctx, task, download.path, download.size, download.digest, probed)
	if err != nil {
		// A lost lease is not one of the answers here. `commit` speaks to the object
		// store and to nothing that holds the lease, so every failure it can report is
		// one of its own steps'; the last lease check in a preparation is the
		// completion write, below.
		failBeforeReady(commitCode(err), err.Error())
		return
	}

	// The material's projection is the boundary. Up to and including this write a
	// failure means the material is not prepared, and all three rows have to be told;
	// the object may already be at the formal key, but nothing points at it and the
	// next attempt writes the same one, because the key is its own bytes' digest.
	if err := p.Materials.MarkVideoReady(task.TeamID, task.AssetID, facts); err != nil {
		failBeforeReady(codeCommitVerify, err.Error())
		return
	}
	// Past it the material is prepared and says so, so every remaining write is
	// bookkeeping on rows that are already correct in the ways the user can see. A
	// failure here must not be recorded as a failure of the preparation.
	p.finish(task, download, facts)
}

// finish releases the downloads waiting on a prepared material and closes the task.
//
// It runs only once the material says it is ready, which is why it takes no part in
// the failure handling above: `MarkVideoFailed` would refuse to take back a `ready`
// projection by its own predicate, and a caller that might need it would be a caller
// that did not know which half it was in.
func (p *Preparer) finish(task transfermodel.Task, download download, facts producerepo.VideoFacts) {
	if _, err := p.Transfers.HandOverDependencies(task.ID, transferrepo.DependencyFacts{
		SourceObjectKey: facts.ObjectKey,
		TotalBytes:      facts.SizeBytes,
		ExpectedSHA256:  facts.SHA256,
	}, p.now()); err != nil {
		// The source is prepared and the material says so, so the waiters are told the
		// hand-over failed rather than that the source did. Their user's retry does not
		// re-fetch anything: the click sees a `ready` material and queues a download of
		// the object that already exists.
		p.failTaskAndWaiters(task, codeHandoverFailed, err.Error())
		return
	}
	completed, err := p.Transfers.CompleteTask(transferrepo.CompletionInput{
		TaskID: task.ID,
		NodeID: p.WorkerID,
		Bytes:  download.size,
		SHA256: download.digest,
	}, p.now())
	if err != nil {
		// Deliberately no terminal write. Everything the user can see is already
		// correct, and recording a failure would make a successful preparation read as
		// a failed one. The row stays `running`, its lease lapses, and the next claim
		// re-runs a preparation whose every step is idempotent because the object is
		// at a key its own bytes derive.
		p.record(task, "completion write failed", err.Error())
		return
	}
	if !completed {
		// The lease lapsed or a cancellation arrived while the bytes were written.
		// `settleLostLease` reads the row to find out which, and releases nothing it
		// should not: the hand-over above cleared the waiters' pointer, so its own
		// release matches no rows.
		p.settleLostLease(task, errLeaseLost)
		return
	}
	p.record(task, "success", "")
}

// download fetches the source to a file outside the checkout, hashing it as it
// arrives.
//
// The heartbeat is driven by the byte flow rather than by a timer of its own. That
// is the whole of this worker's liveness handling: a source that stalls stops
// renewing the lease, the lease lapses, and the task becomes claimable again —
// which is the outcome a stall should have. A timer would keep the lease alive
// while nothing was arriving, and the row would read `running` for as long as the
// worker lived.
func (p *Preparer) download(ctx context.Context, task transfermodel.Task, address string) (download, error) {
	dir, err := os.MkdirTemp(p.TempDir, "wt-media-prepare-")
	if err != nil {
		return download{}, fmt.Errorf("stage a temporary download directory: %w", err)
	}
	// The directory is not removed here, because the file in it is the download: it
	// has to outlive this call for the probe and the upload to read it. The removal
	// belongs to the caller, and a `defer` on this function would delete the bytes
	// the moment there was something worth keeping.
	discard := func() { _ = os.RemoveAll(dir) }

	body, source, err := p.Sources.Open(ctx, address, 0)
	if err != nil {
		discard()
		return download{}, err
	}
	defer body.Close()

	path := filepath.Join(dir, "source")
	file, err := os.Create(path)
	if err != nil {
		discard()
		return download{}, fmt.Errorf("create the staged file: %w", err)
	}
	hasher := sha256.New()
	size, copyErr := p.copyBody(ctx, task, body, io.MultiWriter(file, hasher))
	closeErr := file.Close()
	if copyErr != nil {
		discard()
		return download{}, copyErr
	}
	if closeErr != nil {
		discard()
		return download{}, fmt.Errorf("close the staged file: %w", closeErr)
	}
	if size == 0 {
		discard()
		return download{}, errors.New("the source answered with no bytes")
	}
	// A body shorter than the length the server declared is a truncated download
	// that would otherwise be hashed and committed as if it were the whole video. A
	// server that declared nothing — a chunked answer, which `sourcefile` reports as
	// a negative total — leaves nothing to compare against, and the size that did
	// arrive is then what the object store is asked to confirm.
	if source.TotalBytes >= 0 && size != source.TotalBytes {
		discard()
		return download{}, fmt.Errorf("the source declared %d bytes and delivered %d", source.TotalBytes, size)
	}
	// Offset zero asks for the whole file, so a body that starts anywhere else is an
	// answer to a question this worker did not ask.
	if source.RangeStart != 0 {
		discard()
		return download{}, fmt.Errorf("the source answered from offset %d, not from the beginning", source.RangeStart)
	}
	return download{dir: dir, path: path, size: size, digest: hex.EncodeToString(hasher.Sum(nil))}, nil
}

// copyBody moves the body to the writer, renewing the lease as bytes arrive and
// stopping when the lease is refused.
func (p *Preparer) copyBody(ctx context.Context, task transfermodel.Task, body io.Reader, writer io.Writer) (int64, error) {
	buffer := make([]byte, copyBufferSize)
	interval := p.lease() / 3
	last := p.now()
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return size, err
		}
		read, readErr := body.Read(buffer)
		if read > 0 {
			written, writeErr := writer.Write(buffer[:read])
			size += int64(written)
			if writeErr != nil {
				return size, fmt.Errorf("write the staged file: %w", writeErr)
			}
			if now := p.now(); now.Sub(last) >= interval {
				held, err := p.Transfers.HeartbeatTask(task.ID, p.WorkerID, now, p.lease())
				if err != nil {
					// A heartbeat that could not be sent is a real failure, not a lost
					// lease: nothing said this worker had been replaced. The task fails,
					// and the next attempt starts over.
					return size, fmt.Errorf("renew the transfer lease: %w", err)
				}
				if !held {
					return size, errLeaseLost
				}
				last = now
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return size, nil
			}
			return size, fmt.Errorf("read the source body: %w", readErr)
		}
	}
}

// probeFile opens the staged bytes for the probe and closes them again.
//
// The handle is not held across the upload: `Put` reopens the file by path, so
// keeping one open would only be a descriptor whose lifetime has to be got right
// on every path out of here.
func (p *Preparer) probeFile(path string, size int64) (media.Media, error) {
	file, err := os.Open(path)
	if err != nil {
		return media.Media{}, fmt.Errorf("reopen the staged file for probing: %w", err)
	}
	defer file.Close()
	return p.Probe(file, size)
}

// commit puts the verified bytes in the object store and returns the facts to
// write down.
//
// The file goes to a staging key first, is read back for its size, is copied to
// the key its own digest derives, and is read back again before the staging object
// is removed. The two readings are the same check at the two moments something
// could have gone wrong, and they are the only integrity the store can be asked
// for: `Stat` answers a size and an ETag, and the ETag is the store's own digest
// under a different algorithm — comparing it with this sha256 would be comparing
// two different sums and calling the match meaningful.
//
// The probe has already run, on the bytes on disk. It runs before any of this
// because a container that cannot be parsed is not worth two uploads and an object
// left at a key nothing refers to. The plan's stated order put the probe after the
// upload; the requirement it was protecting — never write a material's facts for
// bytes that did not pass verification at the formal key — is unchanged.
func (p *Preparer) commit(ctx context.Context, task transfermodel.Task, path string, size int64, digest string, probed media.Media) (producerepo.VideoFacts, error) {
	// The container the probe read names the extension. `Probe` answers `mp4` for
	// every file it accepts, and reading it from there rather than from the URL is
	// what keeps the key's suffix a fact about the bytes.
	extension := strings.TrimSpace(probed.Container)
	stagingKey, err := storage.StagingKey(task.AssetID, task.ID, extension)
	if err != nil {
		return producerepo.VideoFacts{}, err
	}
	formalKey, err := storage.SourceKey(task.AssetID, digest, extension)
	if err != nil {
		return producerepo.VideoFacts{}, err
	}

	if err := p.put(ctx, path, stagingKey, size, extension); err != nil {
		return producerepo.VideoFacts{}, fmt.Errorf("%w: %v", errStagingFailed, err)
	}
	if err := p.confirm(ctx, stagingKey, size); err != nil {
		return producerepo.VideoFacts{}, fmt.Errorf("%w: %v", errStagingVerify, err)
	}
	if err := p.Store.Copy(ctx, stagingKey, formalKey); err != nil {
		return producerepo.VideoFacts{}, fmt.Errorf("%w: %v", errCopyFailed, err)
	}
	if err := p.confirm(ctx, formalKey, size); err != nil {
		return producerepo.VideoFacts{}, fmt.Errorf("%w: %v", errCommitVerify, err)
	}
	// The staging object is removed last, and failing to remove it is not a failure
	// of the preparation: the formal key is verified and the facts about to be
	// written name it. An object left under `.staging/` is cheaper than telling an
	// operator a preparation failed that did not.
	if err := p.Store.Remove(ctx, stagingKey); err != nil {
		p.record(task, "committed", "the staging object could not be removed")
	}
	mediaJSON, err := json.Marshal(probed)
	if err != nil {
		return producerepo.VideoFacts{}, fmt.Errorf("encode the probe's reading: %w", err)
	}
	return producerepo.VideoFacts{
		ObjectKey: formalKey,
		SizeBytes: size,
		SHA256:    digest,
		Media:     mediaJSON,
	}, nil
}

func (p *Preparer) put(ctx context.Context, path, key string, size int64, extension string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reopen the staged file for upload: %w", err)
	}
	defer file.Close()
	// The content type is built from the same container the key's suffix is, so the
	// two cannot disagree about what was uploaded.
	return p.Store.Put(ctx, key, file, size, "video/"+extension)
}

func (p *Preparer) confirm(ctx context.Context, key string, size int64) error {
	info, err := p.Store.Stat(ctx, key)
	if err != nil {
		return fmt.Errorf("read back %s: %w", key, err)
	}
	if info.Size != size {
		return fmt.Errorf("%s holds %d bytes, but %d were uploaded", key, info.Size, size)
	}
	return nil
}

// failTaskAndWaiters records a task's terminal failure and releases the downloads
// waiting on it.
//
// The two writes are separate statements, and the waiters are released even when
// the task's own write does not land. Without that, a waiter would sit `pending`
// pointing at a task that is terminal: nothing would ever hand it facts, and
// `leaseable` refuses a local task whose dependency is set, so nothing would pick
// it up either. The reconciliation sweep closes that hole eventually, but eventual
// is a property of a job this build has not registered yet.
func (p *Preparer) failTaskAndWaiters(task transfermodel.Task, code, message string) {
	message = failureText(message)
	if _, err := p.Transfers.FailDependents(task.ID, code, message, p.now()); err != nil {
		p.record(task, "dependent release failed", err.Error())
	}
	written, err := p.Transfers.FailTask(transferrepo.FailureInput{
		TaskID:       task.ID,
		NodeID:       p.WorkerID,
		Status:       transfermodel.StatusFailed,
		ErrorCode:    code,
		ErrorMessage: message,
	}, p.now())
	if err != nil {
		p.record(task, "task failure write failed", err.Error())
		return
	}
	if !written {
		// Another worker owns the row now, or it is already terminal. Its own run
		// decides what it becomes, and this one must not overwrite that.
		p.record(task, "failure not recorded", "the task is no longer running under this worker")
		return
	}
	p.record(task, "failed", code)
}

// settleLostLease decides what a task becomes when this worker is no longer the
// one holding it.
//
// The row itself says which case it is: a cancellation request that arrived while
// the bytes were being written means the terminal state is `cancelled`, and
// anything else means the lease lapsed and the task was retried elsewhere or is
// about to be. Guessing between them would put the wrong word in the downloads
// list, and telling them apart is one read.
func (p *Preparer) settleLostLease(task transfermodel.Task, cause error) {
	current, err := p.Transfers.GetTask(task.ID)
	if err != nil {
		p.record(task, "lease lost", err.Error())
		return
	}
	status, code := transfermodel.StatusFailed, codeLeaseLost
	if current.CancelRequestedAt != nil {
		status, code = transfermodel.StatusCancelled, codeCancelled
	}
	// The waiters are released either way. When this worker was replaced mid-download
	// nothing was ever handed over, so they would otherwise be left pointing at a
	// task that is now terminal. When it got as far as the hand-over, the pointer is
	// already NULL and this matches no rows.
	message := failureText(cause.Error())
	if _, err := p.Transfers.FailDependents(task.ID, code, message, p.now()); err != nil {
		p.record(task, "dependent release failed", err.Error())
	}
	written, err := p.Transfers.FailTask(transferrepo.FailureInput{
		TaskID:       task.ID,
		NodeID:       p.WorkerID,
		Status:       status,
		ErrorCode:    code,
		ErrorMessage: message,
	}, p.now())
	if err != nil {
		// The row's state is not known, so the material is left alone: a task that is
		// still running under a retry can still deliver, and taking the projection back
		// under it would tell the user their material is unprepared while a preparation
		// is on its way. This is the one path where the material is not corrected, and
		// it is recorded rather than passed over.
		p.record(task, "lease lost", err.Error())
		return
	}
	if status == transfermodel.StatusCancelled {
		p.takeBackMaterial(task)
	}
	if !written {
		p.record(task, "lease lost", "the task is no longer running under this worker")
		return
	}
	p.record(task, string(status), code)
}

// takeBackMaterial returns a cancelled preparation's material to `not_downloaded`.
//
// It runs on the cancellation arm of a lost lease, and nothing else corrects this:
// the material's projection says a preparation is in flight, the task that meant is
// cancelled, and a cancelled row is never leasable — so no later attempt will
// deliver one and no later write will notice. Left alone, the operator reads
// 「准备中」 for a preparation that ended, and the click that would restart it does
// not read the projection either (`MarkVideoPreparing` refuses `downloading`), so
// the row describes a state nobody is in.
//
// Not gated on the terminal write landing: a cancellation request makes the row
// un-leasable, so a write that found it out of this worker's hands found it
// terminal. A failure here is recorded rather than returned, because the task row
// is already correct and the material is the half the user can see.
func (p *Preparer) takeBackMaterial(task transfermodel.Task) {
	if err := p.Materials.MarkVideoNotPrepared(task.TeamID, task.AssetID); err != nil {
		p.record(task, "material write failed", err.Error())
	}
}

func (p *Preparer) record(task transfermodel.Task, outcome, detail string) {
	if p.Log == nil {
		return
	}
	p.Log.PreparationFinished(task.ID, task.AssetID, outcome, failureText(detail))
}

func (p *Preparer) lease() time.Duration {
	if p.Lease > 0 {
		return p.Lease
	}
	return transferservice.DefaultLease
}

func (p *Preparer) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

// failureText bounds a failure message to what the column and the repository will
// both accept, cutting on a character boundary: a byte-wise cut through the middle
// of a multi-byte character writes a broken one into the row, and people read the
// row.
func failureText(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= failureTextLimit {
		return message
	}
	// The ellipsis is itself three bytes and is part of the bound.
	limit := failureTextLimit - len("…")
	cut := 0
	for index := range message {
		if index > limit {
			break
		}
		cut = index
	}
	return message[:cut] + "…"
}

// commitCode names which step of the commit refused, so that the operator's list
// says more than "the upload failed".
func commitCode(err error) string {
	switch {
	case errors.Is(err, errStagingFailed):
		return codeStagingFailed
	case errors.Is(err, errCopyFailed):
		return codeCopyFailed
	case errors.Is(err, errCommitVerify):
		return codeCommitVerify
	default:
		return codeStagingVerify
	}
}

type download struct {
	// dir is the temporary directory the file lives in, and the caller removes it.
	dir    string
	path   string
	size   int64
	digest string
}

// copyBufferSize is the download's block size. It bounds how much a single `Read`
// returns, and the heartbeat is checked between blocks.
const copyBufferSize = 64 * 1024

// RunMaterialPrepareWorker runs one preparation for each tick of the worker
// process.
//
// One task per tick, not a batch: a preparation fetches and uploads a video, and a
// worker that claimed several would hold leases on all of them while working
// through one, timing the others out. The scheduler gives each registered job its
// own goroutine and does not overlap a job with itself, so a long download delays
// the next preparation and nothing else.
//
// A process with no object-storage credential does nothing at all, and does so
// before assembling anything. Claiming a task it cannot serve would spend an
// attempt and mark a material failed for a condition only an operator can fix;
// leaving the task pending keeps it claimable once the credential exists. It is
// silent rather than logged once per tick, because a worker that says "not
// configured" every interval is noise and the condition belongs in a startup
// report.
func RunMaterialPrepareWorker(ctx context.Context) error {
	if !storage.Configured() {
		return nil
	}
	if _, err := NewPreparer().Prepare(ctx); err != nil {
		return fmt.Errorf("run a material preparation: %w", err)
	}
	return nil
}

// NewPreparer assembles the worker's real dependencies.
func NewPreparer() *Preparer {
	return &Preparer{
		WorkerID:  cloudWorkerID(),
		Lease:     transferservice.DefaultLease,
		Store:     storage.Get(),
		Sources:   sourcefile.New(),
		Details:   platformMediaAddresses{},
		Materials: productionProjection{},
		Transfers: transferQueue{},
		Probe:     media.Probe,
		Now:       time.Now,
		Log:       jobLog{},
	}
}

// cloudWorkerID names this process in `claimed_by_node_id`, the column every
// conditional update uses to decide who owns a task.
//
// It carries the host, the pid and a random suffix: the first two make a stuck
// lease traceable to a machine, and the suffix keeps two workers on one host from
// being mistaken for each other after a restart — which is exactly the moment a
// task's previous owner is still worth telling apart from its new one.
func cloudWorkerID() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown-host"
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		// A worker that cannot name itself distinctly must still not share a name with
		// the process it replaced, and the pid and the clock still tell them apart.
		return fmt.Sprintf("cloud-worker:%s:%d:%d", host, os.Getpid(), time.Now().UnixNano())
	}
	return fmt.Sprintf("cloud-worker:%s:%d:%s", host, os.Getpid(), hex.EncodeToString(suffix))
}

// productionProjection calls the production module's public functions.
type productionProjection struct{}

func (productionProjection) ResolvePreparationSource(teamID identity.TeamID, materialID int64) (productionservice.PreparationSource, error) {
	return productionservice.ResolvePreparationSource(teamID, materialID)
}

func (productionProjection) MarkVideoReady(teamID identity.TeamID, materialID int64, facts producerepo.VideoFacts) error {
	return productionservice.MarkVideoReady(teamID, materialID, facts)
}

func (productionProjection) MarkVideoFailed(teamID identity.TeamID, materialID int64, message string) error {
	return productionservice.MarkVideoFailed(teamID, materialID, message)
}

func (productionProjection) MarkVideoNotPrepared(teamID identity.TeamID, materialID int64) error {
	return productionservice.MarkVideoNotPrepared(teamID, materialID)
}

// transferQueue calls the transfer module's Cloud-executor repository functions
// directly. They are the executor's own entry points: the service's equivalents
// all begin by authenticating a node credential, which a Cloud worker does not
// have and must not pretend to.
type transferQueue struct{}

func (transferQueue) ClaimCloudTask(workerID string, now time.Time, lease time.Duration) (transfermodel.Task, bool, error) {
	return transferrepo.ClaimCloudTask(workerID, now, lease)
}

func (transferQueue) GetTask(taskID string) (transfermodel.Task, error) {
	return transferrepo.GetTask(taskID)
}

func (transferQueue) HeartbeatTask(taskID, nodeID string, now time.Time, lease time.Duration) (bool, error) {
	return transferrepo.HeartbeatTask(taskID, nodeID, now, lease)
}

func (transferQueue) HandOverDependencies(prepareTaskID string, facts transferrepo.DependencyFacts, now time.Time) (int64, error) {
	return transferrepo.HandOverDependencies(prepareTaskID, facts, now)
}

func (transferQueue) CompleteTask(input transferrepo.CompletionInput, now time.Time) (bool, error) {
	return transferrepo.CompleteTask(input, now)
}

func (transferQueue) FailTask(input transferrepo.FailureInput, now time.Time) (bool, error) {
	return transferrepo.FailTask(input, now)
}

func (transferQueue) FailDependents(prepareTaskID, errorCode, errorMessage string, now time.Time) (int64, error) {
	return transferrepo.FailDependents(prepareTaskID, errorCode, errorMessage, now)
}

// platformMediaAddresses resolves a source address from the platform that named
// it. One platform is implemented, and any other is refused rather than guessed
// at: the platforms are not interchangeable and the field that holds a playable
// address differs between them.
type platformMediaAddresses struct{}

func (platformMediaAddresses) Address(ctx context.Context, platform string, contentID int64) (string, error) {
	if strings.TrimSpace(platform) != douyinPlatform {
		return "", fmt.Errorf("platform %q has no address reader", platform)
	}
	client := douyin.Get()
	if !client.Configured() {
		// A client with no API key would send an unauthenticated request and read the
		// provider's refusal as "this video has no address", which is the same answer
		// a real absence gives. Refusing here keeps the two apart.
		return "", errors.New("the douyin client is not configured")
	}
	// The material's stable numeric content id is the identity, not the snapshot's
	// stored short link: a detail request by id is one call, and the id is what the
	// crawler stored as `platform_content_id`.
	payload, err := client.FetchByURL(ctx, douyin.FetchByURLRequest{URL: strconv.FormatInt(contentID, 10)})
	if err != nil {
		return "", err
	}
	item, err := douyin.DetailItem(payload)
	if err != nil {
		return "", err
	}
	return douyin.MediaAddress(item)
}

// douyinPlatform is the platform name a material carries, and the only one this
// build can prepare. It is the same literal `contentpool`'s crawler refuses a
// discovery request against, spelled here rather than imported from a module that
// would then be a dependency of a worker for one string.
const douyinPlatform = "douyin"

// jobLog writes through the job logger, where every other job's output goes.
type jobLog struct{}

func (jobLog) PreparationFinished(taskID string, materialID int64, outcome string, detail string) {
	logger.Job().Infof(
		"material preparation task_id=%s material_id=%d outcome=%s detail=%s",
		taskID,
		materialID,
		outcome,
		detail,
	)
}
