// Package service owns material visibility and the “My Materials” use case.
package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
)

var (
	ErrInvalidInput = errors.New("production input is invalid")
	ErrForbidden    = errors.New("production operation is forbidden")
	ErrNotFound     = errors.New("material not found")

	// ErrUsageNotFound is a missing "My Materials" row, which is a different
	// answer from a missing material: the two carry different errcodes and
	// different copy in the UI. It is a separate sentinel rather than a wrapper
	// around ErrNotFound precisely so the handler can keep them apart.
	ErrUsageNotFound = errors.New("material usage not found")

	// ErrUsageForbidden is a "My Materials" row the actor may not act on, as
	// opposed to a material the actor may not see. Both are 403 and both are
	// about the same relation, but they are refusals of different things — one
	// of the row, one of the material behind it — and the frozen error names
	// distinguish them.
	ErrUsageForbidden = errors.New("material usage operation is forbidden")

	// ErrMaterialUnavailable is "the material is visible to you but has no
	// prepared video yet", which is a 409 and not a 404: the material exists and
	// the answer will change on its own once preparation finishes.
	ErrMaterialUnavailable = errors.New("material has no prepared video")

	// ErrLocalNodeUnavailable is "there is no fresh Local Agent on this account to
	// download to". It is a separate sentinel from the runtime-binding module's
	// `ErrLocalTrustUnavailable` so that the handler maps a production decision
	// rather than a foreign error, and so the two modules stay free to disagree
	// about what that condition is called.
	ErrLocalNodeUnavailable = errors.New("no fresh local agent node is available")
)

type Store interface {
	FindMaterial(int64) (model.Material, bool, error)
	ListMaterials(repository.MaterialFilter) ([]model.Material, error)
	CreateOrRestoreUsage(repository.CreateUsageInput, time.Time) (model.MaterialUsage, bool, error)
	ListActiveUsages(identityservice.UserID) ([]model.MaterialUsage, error)
	FindUsageForUser(int64, identityservice.UserID) (model.MaterialUsage, bool, error)
	RemoveUsageByID(int64, identityservice.UserID, time.Time) (bool, error)

	// MarkVideoPreparing writes the material's readiness projection for the start
	// of a preparation, and reports whether the row was in a state that allowed it.
	// The flag is part of the interface rather than an implementation detail
	// because the caller's next step depends on it.
	MarkVideoPreparing(identityservice.TeamID, int64, time.Time) (bool, error)

	// MarkVideoReady writes the facts of a verified prepared source, and reports
	// whether the scope matched a row at all.
	MarkVideoReady(identityservice.TeamID, int64, repository.VideoFacts, time.Time) (bool, error)

	// MarkVideoFailed records that a preparation produced no verified source. Its
	// flag is false for a material that is already `ready`, which is a refusal the
	// caller must not read as "the row was not there".
	MarkVideoFailed(identityservice.TeamID, int64, string, time.Time) (bool, error)
}

// LocalNodeResolver answers which of the actor's machines should receive a file.
//
// A consumer-side interface, one method wide, implemented in store_adapter.go —
// the shape profileguard and filetransfer already use. `production` is allowed to
// depend on the runtime-binding domain in this direction; declaring the question
// here rather than calling the module directly keeps the dependency to one call
// site and lets a test answer it without a node table.
type LocalNodeResolver interface {
	ResolveFreshLocalNode(identityservice.UserID) (runtimeservice.AgentNode, error)
}

// TransferCreator queues the task a Local Agent will later claim.
//
// The direction matters and is asserted by a test in the other module: the
// transfer module may not import `production`, so the material facts a download
// needs are copied into the task here, at creation, rather than read back by the
// executor.
type TransferCreator interface {
	CreateUserDownload(transferservice.CreateUserDownloadInput) (transferdto.Task, error)

	// EnsureMaterialSourcePrepare queues the Cloud task that produces the video a
	// waiting download needs, and answers with the one already outstanding when
	// there is one. The caller cannot tell the difference and must not need to.
	EnsureMaterialSourcePrepare(transferservice.EnsureMaterialSourcePrepareInput) (transferdto.Task, error)
}

type Service struct {
	store     Store
	nodes     LocalNodeResolver
	transfers TransferCreator
	now       func() time.Time
}

func NewService(store Store, nodes LocalNodeResolver, transfers TransferCreator) *Service {
	return &Service{store: store, nodes: nodes, transfers: transfers, now: time.Now}
}

func (s *Service) GetMaterial(actor identityservice.PublicUser, materialID int64) (model.Material, error) {
	if materialID <= 0 {
		return model.Material{}, ErrInvalidInput
	}
	material, found, err := s.store.FindMaterial(materialID)
	if err != nil {
		return model.Material{}, err
	}
	if !found {
		return model.Material{}, ErrNotFound
	}
	if !canAccessMaterial(actor, material) {
		return model.Material{}, ErrForbidden
	}
	return material, nil
}

// AddUsage adds the material to the actor's library and reports whether that
// changed anything.
//
// A repeat click is not a failure and is not a creation either. The two are
// distinguished because the frozen contract publishes both 200 and 201 for this
// route, and the difference is only knowable here: the row that comes back looks
// identical either way, so a caller that had to infer it from the body would be
// guessing.
func (s *Service) AddUsage(actor identityservice.PublicUser, materialID int64) (model.MaterialUsage, bool, error) {
	material, err := s.GetMaterial(actor, materialID)
	if err != nil {
		return model.MaterialUsage{}, false, err
	}
	usage, created, err := s.store.CreateOrRestoreUsage(repository.CreateUsageInput{TeamID: material.TeamID, MaterialID: material.ID, UserID: actor.ID}, s.now().UTC())
	if err != nil {
		return model.MaterialUsage{}, false, err
	}
	return usage, created, nil
}

func (s *Service) ListMaterials(actor identityservice.PublicUser, search string) ([]model.Material, error) {
	if actor.Status != identityservice.UserStatusEnabled {
		return nil, ErrForbidden
	}
	filter := repository.MaterialFilter{Search: strings.TrimSpace(search)}
	if actor.Role != identityservice.RoleAdmin {
		if actor.TeamID == nil || len(actor.GameIDs) == 0 {
			return nil, ErrForbidden
		}
		filter.TeamID = actor.TeamID
		filter.GameIDs = append([]string(nil), actor.GameIDs...)
	}
	return s.store.ListMaterials(filter)
}

func (s *Service) ListMyMaterials(actor identityservice.PublicUser) ([]model.MaterialUsage, error) {
	if actor.Status != identityservice.UserStatusEnabled || actor.ID <= 0 {
		return nil, ErrForbidden
	}
	usages, err := s.store.ListActiveUsages(actor.ID)
	if err != nil {
		return nil, err
	}
	visible := make([]model.MaterialUsage, 0, len(usages))
	for _, usage := range usages {
		material, found, err := s.store.FindMaterial(usage.MaterialID)
		if err != nil {
			return nil, err
		}
		if !found || !canAccessMaterial(actor, material) {
			continue
		}
		usage.Material = &material
		visible = append(visible, usage)
	}
	return visible, nil
}

func (s *Service) RemoveUsage(actor identityservice.PublicUser, usageID int64) error {
	if usageID <= 0 || actor.ID <= 0 {
		return ErrInvalidInput
	}
	usage, found, err := s.store.FindUsageForUser(usageID, actor.ID)
	if err != nil {
		return err
	}
	if !found {
		return ErrUsageNotFound
	}
	material, err := s.GetMaterial(actor, usage.MaterialID)
	if err != nil {
		return err
	}
	if material.TeamID != usage.TeamID {
		return ErrUsageForbidden
	}
	removed, err := s.store.RemoveUsageByID(usageID, actor.ID, s.now().UTC())
	if err != nil {
		return err
	}
	if !removed {
		// The row left the actor's scope between the lookup and the update — the
		// conditional `status = 'active'` in the same statement is what reports
		// it. That is the same outcome as never having had it.
		return ErrUsageNotFound
	}
	return nil
}

// CreateDownload queues the actor's download of one material, whether or not its
// video is ready yet.
//
// The user clicks once, and what that click means depends on the material — but it
// is one task either way. A prepared material gets a `user_download` the actor's
// own machine can claim immediately. A material whose video is not ready gets the
// same kind of task, held back by `dependency_task_id` until the Cloud
// preparation it names writes the object key, the size and the hash it waits for.
//
// That is why nothing here is called a "cloud task": the two statuses the user is
// looking at are two rows, and the coupling between them is that one column. The
// machine's polling is then part of its own download rather than a second
// mechanism — a claim on a waiting task answers "nothing to do" until the
// preparation hands the facts over, and the same task becomes claimable without
// the user clicking again.
//
// The refusals are ordered by what the user can do about them: a material outside
// the actor's scope is not theirs to see (the 404/403 from `GetMaterial`), a
// visible material with no prepared video and no source to prepare from is one
// nothing can be done about, and no fresh node is one they can fix by opening the
// app on a machine. Answering 409 for the last two rather than 500 is the whole
// point of asking here.
//
// The download is **never** queued without a node. An executor claims on its own
// credential, so a task with no assignee would be claimed by whichever device
// polled first — a file the user asked for on one machine landing on another.
func (s *Service) CreateDownload(actor identityservice.PublicUser, materialID int64) (transferdto.Task, error) {
	material, err := s.GetMaterial(actor, materialID)
	if err != nil {
		return transferdto.Task{}, err
	}
	ready := material.VideoStatus == model.VideoReady
	var objectKey, sha256 string
	var size int64
	switch {
	case ready:
		if objectKey, size, sha256, err = videoFacts(material); err != nil {
			return transferdto.Task{}, err
		}
	case strings.TrimSpace(material.SourceURL) == "":
		// Not ready *and* nothing to prepare from: the material's source address is
		// gone, so waiting produces no file and the 409 is honest. Every other
		// not-ready state is a wait rather than a refusal, which is why the
		// condition is this narrow rather than `!ready`.
		return transferdto.Task{}, ErrMaterialUnavailable
	}
	node, err := s.nodes.ResolveFreshLocalNode(actor.ID)
	if err != nil {
		// Everything the resolver refuses means the same thing here, including a
		// store failure: from this route's side there is no node to download to.
		// The distinction is kept in the log, not in the response.
		return transferdto.Task{}, ErrLocalNodeUnavailable
	}
	input := transferservice.CreateUserDownloadInput{
		TeamID:          material.TeamID,
		AssetID:         material.ID,
		AssetTitle:      material.Title,
		SourceObjectKey: objectKey,
		RequestedBy:     actor.ID,
		AssignedNodeID:  node.ID,
		TotalBytes:      size,
		ExpectedSHA256:  sha256,
	}
	if !ready {
		// The preparation is a Cloud task keyed on the material rather than on this
		// user: two operators clicking at once get one fetch, and the second click's
		// wait attaches to the task the first one queued. `transfer`'s dedupe is what
		// makes that true, so this call needs no guard here.
		prepare, err := s.transfers.EnsureMaterialSourcePrepare(transferservice.EnsureMaterialSourcePrepareInput{
			TeamID:      material.TeamID,
			AssetID:     material.ID,
			AssetTitle:  material.Title,
			RequestedBy: actor.ID,
		})
		if err != nil {
			return transferdto.Task{}, fmt.Errorf("queue material source preparation for material %d: %w", material.ID, err)
		}
		input.DependencyTaskID = prepare.ID
		// The task is the fact and the projection follows it, in that order: a
		// `downloading` material with no task would show the user a queue position
		// that does not exist. The statement is guarded so that this click cannot
		// take a `ready` material back to `downloading`.
		preparing, err := s.store.MarkVideoPreparing(material.TeamID, material.ID, s.now().UTC())
		if err != nil {
			return transferdto.Task{}, fmt.Errorf("mark material %d preparing: %w", material.ID, err)
		}
		if !preparing {
			// The guard refused, so the material left `not_downloaded`/`failed`
			// between the read at the top of this method and the statement above.
			// There are two ways that happens and they need different tasks: another
			// click is already preparing this material, in which case waiting for it
			// is exactly the plan, or preparation finished and it is now `ready`, in
			// which case a wait would hold a file back that can be fetched now. Only
			// a fresh read tells the two apart, and guessing wrong on the second is
			// the one that strands a download.
			if material, err = s.GetMaterial(actor, materialID); err != nil {
				return transferdto.Task{}, err
			}
			if material.VideoStatus == model.VideoReady {
				input.DependencyTaskID = ""
				if input.SourceObjectKey, input.TotalBytes, input.ExpectedSHA256, err = videoFacts(material); err != nil {
					return transferdto.Task{}, err
				}
			}
		}
	}
	task, err := s.transfers.CreateUserDownload(input)
	if err != nil {
		if errors.Is(err, transferservice.ErrInvalidInput) {
			return transferdto.Task{}, ErrInvalidInput
		}
		return transferdto.Task{}, fmt.Errorf("queue user download for material %d: %w", material.ID, err)
	}
	return task, nil
}

// MarkVideoReady records the facts of a verified prepared source. It is the Cloud
// worker's write rather than an HTTP use case: its scope comes from the task row
// and not from an actor, and no route reaches it.
//
// A write that changed no row is reported as a missing material rather than as a
// flag, because that is the only reading available. The statement carries no state
// predicate — a preparation may finish from any state — and it writes a fresh
// `updated_at` on every call, so a row it matched is always a row it changed. Zero
// therefore means the scope matched nothing: the task names a material that is gone
// or belongs to another team. Reporting that as a completed preparation would leave
// a download waiting on a video nobody is preparing, so it is an error here.
//
// The MySQL connection does not set `CLIENT_FOUND_ROWS`, which is what makes
// "changed" the right word: with it set, the same reading would still hold, and
// without it a statement that set nothing new would report zero.
func (s *Service) MarkVideoReady(teamID identityservice.TeamID, materialID int64, facts repository.VideoFacts) error {
	written, err := s.store.MarkVideoReady(teamID, materialID, facts, s.now().UTC())
	if err != nil {
		return err
	}
	if !written {
		return fmt.Errorf("%w: material %d in team %d", ErrNotFound, materialID, teamID)
	}
	return nil
}

// MarkVideoFailed records that a preparation produced no verified source.
//
// Unlike `MarkVideoReady`, a write that changed no row is not an error. The
// statement refuses a material that is already `ready`, and a preparation failing
// after a slower earlier attempt succeeded is exactly the case that refusal exists
// for: the task still fails, and the material keeps the video it has. A material
// that is not there at all reads the same way and needs no answer either — there is
// no projection left to correct.
func (s *Service) MarkVideoFailed(teamID identityservice.TeamID, materialID int64, message string) error {
	_, err := s.store.MarkVideoFailed(teamID, materialID, message, s.now().UTC())
	return err
}

// PreparationSource names the platform content a material's video has to be
// re-resolved from at execution time.
//
// The worker must not read the address out of `materials.source_snapshot`: that
// column holds the provider's raw payload, whose play address is a signed URL and
// is expired by the time anything downloads it. What is stable is the platform and
// the provider's own content id, so those are what this carries, and the fetch
// happens fresh on every attempt.
//
// The scope is the team from the task row rather than an actor, because the worker
// has no session. A material in another team reads as missing: the worker is not
// being refused on anyone's behalf, it is being told the task does not name a
// material this team has, and `ErrNotFound` is the same answer it would get for an
// id that never existed.
type PreparationSource struct {
	TeamID     identityservice.TeamID
	MaterialID int64
	Platform   string
	ContentID  int64
}

func (s *Service) PreparationSource(teamID identityservice.TeamID, materialID int64) (PreparationSource, error) {
	if teamID <= 0 || materialID <= 0 {
		return PreparationSource{}, ErrInvalidInput
	}
	material, found, err := s.store.FindMaterial(materialID)
	if err != nil {
		return PreparationSource{}, err
	}
	if !found || material.TeamID != teamID {
		return PreparationSource{}, fmt.Errorf("%w: material %d in team %d", ErrNotFound, materialID, teamID)
	}
	// The id the worker sends is the provider's own, not this row's
	// `source_contents.id`. The provider has never heard of the latter, and asked
	// for it answers that the video does not exist -- which reads exactly like a
	// video that really is gone, so the mistake is silent until a real run.
	// `SourceContentID` is what the projection joins on; it is not an identity the
	// platform knows, and the two are separate columns for that reason.
	platform := strings.TrimSpace(material.Platform)
	contentID, parseErr := strconv.ParseInt(strings.TrimSpace(material.PlatformContentID), 10, 64)
	if platform == "" || parseErr != nil || contentID <= 0 {
		return PreparationSource{}, fmt.Errorf("material %d names no platform source to re-resolve", materialID)
	}
	return PreparationSource{
		TeamID:     material.TeamID,
		MaterialID: material.ID,
		Platform:   platform,
		ContentID:  contentID,
	}, nil
}

// videoFacts is the three facts a local download needs out of a `ready` row.
//
// `ready` is the projection's promise that the object key, the size and the hash
// were all written in the same step. A ready row missing one of them is this
// module's own inconsistency, so it is reported as a fault rather than as the 409
// the user would read as "try again later".
func videoFacts(material model.Material) (string, int64, string, error) {
	if material.SourceObjectKey == "" || material.VideoSizeBytes == nil || *material.VideoSizeBytes <= 0 || material.VideoSHA256 == "" {
		return "", 0, "", fmt.Errorf("material %d is ready but its video facts are incomplete", material.ID)
	}
	return material.SourceObjectKey, *material.VideoSizeBytes, material.VideoSHA256, nil
}

func canAccessMaterial(actor identityservice.PublicUser, material model.Material) bool {
	if actor.Status != identityservice.UserStatusEnabled {
		return false
	}
	if actor.Role == identityservice.RoleAdmin {
		return true
	}
	if actor.TeamID == nil || *actor.TeamID != material.TeamID || material.GameID == nil {
		return false
	}
	for _, gameID := range actor.GameIDs {
		if strings.TrimSpace(gameID) == *material.GameID {
			return true
		}
	}
	return false
}
