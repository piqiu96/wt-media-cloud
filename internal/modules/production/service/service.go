// Package service owns material visibility and the “My Materials” use case.
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	transfermodel "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/model"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
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
)

type Store interface {
	FindMaterial(int64) (model.Material, bool, error)
	ListMaterials(repository.MaterialFilter) ([]model.Material, error)
	// ListUsageRelations and ListMaterialOutputFacts fill the library's three
	// usage dimensions in one batch per page rather than one query per row.
	ListUsageRelations(identityservice.UserID, []int64) ([]repository.MaterialUsageRelation, error)
	ListMaterialOutputFacts([]int64) ([]repository.MaterialOutputFact, error)
	// HasActiveUsage is the personal gate on downloading a material.
	HasActiveUsage(int64, identityservice.UserID) (bool, error)
	CreateOrRestoreUsage(repository.CreateUsageInput, time.Time) (model.MaterialUsage, bool, error)
	ListUsages(identityservice.UserID) ([]model.MaterialUsage, error)
	FindUsageForUser(int64, identityservice.UserID) (model.MaterialUsage, bool, error)
	RemoveUsageByID(int64, identityservice.UserID, time.Time) (bool, error)

	// RestoreUsageByID is the inverse of RemoveUsageByID. It reports no flag: the
	// one outcome a zero-row result can describe is "the row was already back in
	// use", which is the state the caller asked for (see the repository).
	RestoreUsageByID(int64, identityservice.UserID, time.Time) error

	// MarkVideoPreparing writes the material's readiness projection for the start
	// of a preparation, and reports whether the row was in a state that allowed it.
	// The flag is part of the interface rather than an implementation detail
	// because the caller's next step depends on it.
	MarkVideoPreparing(identityservice.TeamID, int64, time.Time) (bool, error)

	// MarkVideoRepreparing moves a `ready` material back to `downloading` after its
	// object was observed missing. The caller names the exact key and prepared
	// timestamp it observed, so a concurrent attempt that finished in between is
	// not taken back; false means the row moved on and the caller must re-read.
	MarkVideoRepreparing(identityservice.TeamID, int64, string, *time.Time, time.Time) (bool, error)

	// MarkVideoReady writes the facts of a verified prepared source, and reports
	// whether the scope matched a row at all.
	MarkVideoReady(identityservice.TeamID, int64, repository.VideoFacts, time.Time) (bool, error)

	// MarkVideoFailed records that a preparation produced no verified source. Its
	// flag is false for a material that is already `ready`, which is a refusal the
	// caller must not read as "the row was not there".
	MarkVideoFailed(identityservice.TeamID, int64, string, time.Time) (bool, error)

	// MarkVideoNotPrepared takes the `downloading` projection back when the
	// preparation it was waiting on was cancelled. Its flag is false for a material
	// that is not `downloading`, which here is the ordinary outcome rather than a
	// refusal: there is nothing to take back.
	MarkVideoNotPrepared(identityservice.TeamID, int64, time.Time) (bool, error)
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

	// LatestUserDownloadStatuses answers each material's newest user_download
	// status for one user, raw (see the transfer service). "My Materials" derives
	// its download lifecycle from it; the transfer module stays out of badges.
	LatestUserDownloadStatuses(identityservice.UserID, []int64) (map[int64]string, error)

	// AttachCompletedDependency releases a download that was inserted after its
	// preparation had already succeeded. The bool reports that the attachment
	// landed; the returned task is the refreshed row.
	AttachCompletedDependency(taskID string) (transferdto.Task, bool, error)
}

// ObjectLinker mints the address a stored object is read at; a consumer-side
// interface in the same shape as `LocalNodeResolver`, so a test can answer it
// without the object-storage registry.
//
// It is a signed, short-lived grant rather than the bucket's stable address:
// the bucket refuses anonymous reads, so an unsigned address is one Cloud
// itself has already proven a browser cannot open. The context is passed
// through for an issuer that has to reach the object store; the one wired today
// signs locally and ignores it.
//
// The expiry is deliberately not part of the answer. It is not in the frozen
// `MaterialVideoLink` body, and the drawer asks for the address at the moment
// the operator clicks, so there is no held address for an expiry to invalidate.
type ObjectLinker interface {
	PresignObjectURL(ctx context.Context, objectKey string) (string, error)

	// ObjectExists answers whether the bucket currently holds the key. `ready` is a
	// projection and a projection can outlive what it points at — the incident this
	// check was added for — so every consumer that is about to promise the object
	// asks first. A missing object is `(false, nil)`; only a store failure is an
	// error, because "cannot tell" must not be reported as "gone".
	ObjectExists(ctx context.Context, objectKey string) (bool, error)
}

type Service struct {
	store     Store
	transfers TransferCreator
	links     ObjectLinker
	now       func() time.Time
	// resolveGame names the game a download is filed under. It is a field so the
	// unit tests can stub the store call; the constructor installs the real one.
	resolveGame func(gameID string) (string, bool, error)
	// operators is the identity directory the usage summary reads names from.
	// It stays nil in unit tests that only exercise counts; the wired service
	// installs the real lookup.
	operators func() ([]identityservice.PublicUser, error)
}

func NewService(store Store, transfers TransferCreator, links ObjectLinker) *Service {
	return &Service{
		store: store, transfers: transfers, links: links, now: time.Now,
		resolveGame: func(gameID string) (string, bool, error) {
			game, ok, err := identityservice.ResolveGame(gameID)
			return game.Name, ok, err
		},
	}
}

// GetMaterial is the actor-facing read: visibility first, then the usage
// dimensions the detail drawer shows. Internal callers that only need the
// material's own facts use getMaterial, so a download or a relation write does
// not pay for a usage summary it never reads.
func (s *Service) GetMaterial(actor identityservice.PublicUser, materialID int64) (model.Material, error) {
	material, err := s.getMaterial(actor, materialID)
	if err != nil {
		return model.Material{}, err
	}
	items, err := s.decorateMaterials(actor, []model.Material{material})
	if err != nil {
		return model.Material{}, err
	}
	return items[0], nil
}

func (s *Service) getMaterial(actor identityservice.PublicUser, materialID int64) (model.Material, error) {
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

// VideoURL answers the cloud address of a prepared material's video — the
// detail drawer's link, deliberately not part of the material body. Scope first,
// then readiness (409 while preparation is unfinished), then the same
// `videoFacts` completeness a local download reads, then a check that the object
// is really in the bucket (409 if not — a `ready` row can outlive its object),
// and only then a signature.
func (s *Service) VideoURL(ctx context.Context, actor identityservice.PublicUser, materialID int64) (string, error) {
	material, err := s.getMaterial(actor, materialID)
	if err != nil {
		return "", err
	}
	if material.VideoStatus != model.VideoReady {
		return "", ErrMaterialUnavailable
	}
	objectKey, _, _, err := videoFacts(material)
	if err != nil {
		return "", err
	}
	exists, err := s.links.ObjectExists(ctx, objectKey)
	if err != nil {
		return "", fmt.Errorf("check material %d source object: %w", material.ID, err)
	}
	if !exists {
		// Signing a missing object would open a 404 in the drawer. The 409 says the
		// same thing the download route says: no prepared video is available right
		// now, and a preparation can change that.
		return "", ErrMaterialUnavailable
	}
	return s.links.PresignObjectURL(ctx, objectKey)
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
	material, err := s.getMaterial(actor, materialID)
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
	return s.ListMaterialsFiltered(actor, repository.MaterialFilter{Search: search})
}

// ListMaterialsFiltered reads the team's public library. A game is a filter,
// not a visibility boundary (§5.3.3): an ordinary operator sees every game in
// their team and may narrow the list to the games they are responsible for.
// The team itself is still enforced server-side — an ordinary or senior
// operator cannot ask for another team, and an admin may name the target team.
func (s *Service) ListMaterialsFiltered(actor identityservice.PublicUser, filter repository.MaterialFilter) ([]model.Material, error) {
	if actor.Status != identityservice.UserStatusEnabled {
		return nil, ErrForbidden
	}
	filter.Search = strings.TrimSpace(filter.Search)
	switch actor.Role {
	case identityservice.RoleAdmin:
		// The target team is whatever the request named; absent means all teams,
		// which is the admin's own reading scope.
	case identityservice.RoleOperator, identityservice.RoleSeniorOperator:
		if actor.TeamID == nil {
			return nil, ErrForbidden
		}
		if filter.TeamID != nil && *filter.TeamID != *actor.TeamID {
			return nil, ErrForbidden
		}
		filter.TeamID = actor.TeamID
	default:
		return nil, ErrForbidden
	}
	filter.UserID = actor.ID
	items, err := s.store.ListMaterials(filter)
	if err != nil {
		return nil, err
	}
	return s.decorateMaterials(actor, items)
}

// decorateMaterials fills the viewer's relation and the team's successful-output
// facts onto a page of materials. Two batch reads cover the page; names are a
// directory read only for the roles allowed to see them.
func (s *Service) decorateMaterials(actor identityservice.PublicUser, items []model.Material) ([]model.Material, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	relations, err := s.store.ListUsageRelations(actor.ID, ids)
	if err != nil {
		return nil, err
	}
	facts, err := s.store.ListMaterialOutputFacts(ids)
	if err != nil {
		return nil, err
	}
	relationsByMaterial := make(map[int64]repository.MaterialUsageRelation, len(relations))
	for _, relation := range relations {
		relationsByMaterial[relation.MaterialID] = relation
	}
	summaryByMaterial := make(map[int64]*model.MaterialUsageSummary, len(ids))
	var operatorIDs []int64
	for _, fact := range facts {
		summary := summaryByMaterial[fact.MaterialID]
		if summary == nil {
			summary = &model.MaterialUsageSummary{}
			summaryByMaterial[fact.MaterialID] = summary
		}
		summary.OutputCount += fact.OutputCount
		if fact.OperatorID == nil {
			continue
		}
		summary.OperatorCount++
		summary.Operators = append(summary.Operators, model.MaterialOperatorUse{ID: *fact.OperatorID, OutputCount: fact.OutputCount})
		operatorIDs = append(operatorIDs, *fact.OperatorID)
	}
	names := map[int64]string(nil)
	if actor.Role == identityservice.RoleSeniorOperator || actor.Role == identityservice.RoleAdmin {
		names, err = composeOperatorNameLookup(s.operators, operatorIDs)
		if err != nil {
			return nil, err
		}
	}
	for i := range items {
		item := &items[i]
		if relation, ok := relationsByMaterial[item.ID]; ok {
			item.UsageStatus = relation.Status
			addedAt := relation.CreatedAt
			item.AddedAt = &addedAt
		}
		item.MyUsage = model.MaterialMyUsageUnused
		summary := summaryByMaterial[item.ID]
		if summary == nil {
			summary = &model.MaterialUsageSummary{}
		}
		item.Usage = summary
		for j := range summary.Operators {
			operator := &summary.Operators[j]
			if operator.ID == int64(actor.ID) {
				item.MyUsage = model.MaterialMyUsageUsed
			}
			operator.Name = names[operator.ID]
		}
		if actor.Role != identityservice.RoleSeniorOperator && actor.Role != identityservice.RoleAdmin {
			summary.Operators = nil
		}
	}
	return items, nil
}

func (s *Service) ListMyMaterials(actor identityservice.PublicUser) ([]model.MaterialUsage, error) {
	if actor.Status != identityservice.UserStatusEnabled || actor.ID <= 0 {
		return nil, ErrForbidden
	}
	usages, err := s.store.ListUsages(actor.ID)
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
	// The download lifecycle is per user, not per material: the same row renders
	// differently for two users who downloaded it at different times. One batch
	// query covers the whole visible list instead of a task poll per row.
	statuses, err := s.transfers.LatestUserDownloadStatuses(actor.ID, materialIDsOf(visible))
	if err != nil {
		return nil, err
	}
	for i := range visible {
		visible[i].DownloadStatus = downloadStatusOf(statuses[visible[i].MaterialID])
	}
	return visible, nil
}

// downloadStatusOf maps a raw user_download task status to the user-facing
// download lifecycle. pending and running are one bucket on purpose (the user
// ruled "排队到传输都算下载中"), and failed/cancelled are one bucket too
// ("准备失败和本地下载失败都属于下载失败"). Any other value — or no value at
// all — leaves the field empty and the UI falls back to the material's
// video_status.
func downloadStatusOf(raw string) string {
	switch transfermodel.Status(raw) {
	case transfermodel.StatusPending, transfermodel.StatusRunning:
		return "downloading"
	case transfermodel.StatusSuccess:
		return "downloaded"
	case transfermodel.StatusFailed, transfermodel.StatusCancelled:
		return "failed"
	default:
		return ""
	}
}

func materialIDsOf(usages []model.MaterialUsage) []int64 {
	ids := make([]int64, 0, len(usages))
	for _, usage := range usages {
		ids = append(ids, usage.MaterialID)
	}
	return ids
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
	material, err := s.getMaterial(actor, usage.MaterialID)
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

// RestoreUsage puts a relation the actor gave up back into use.
//
// The checks are the removal's, in the removal's order, because they refuse the
// same two things: a relation that is not the actor's (answered as absent, so that
// one operator cannot probe another's rows by id) and a material outside the
// actor's game scope.
//
// A relation that is already in use is not a refusal — the click asked for exactly
// that state. The common way to ask twice is a double-click arriving before the
// list reloads, and the answer to the second one is the same as to the first.
func (s *Service) RestoreUsage(actor identityservice.PublicUser, usageID int64) error {
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
	material, err := s.getMaterial(actor, usage.MaterialID)
	if err != nil {
		return err
	}
	if material.TeamID != usage.TeamID {
		return ErrUsageForbidden
	}
	if usage.Status != model.MaterialUsageRemoved {
		return nil
	}
	return s.store.RestoreUsageByID(usageID, actor.ID, s.now().UTC())
}

// CreateDownload queues the actor's download of one material, whether or not its
// video is ready yet.
//
// The user clicks once, and what that click means depends on the material — but it
// is one task either way. A prepared material gets a `user_download` any machine
// the actor is logged in on can claim immediately. A material whose video is not
// ready gets the same kind of task, held back by `dependency_task_id` until the
// Cloud preparation it names writes the object key, the size and the hash it
// waits for.
//
// That is why nothing here is called a "cloud task": the two statuses the user is
// looking at are two rows, and the coupling between them is that one column. The
// machine's polling is then part of its own download rather than a second
// mechanism — a claim on a waiting task answers "nothing to do" until the
// preparation hands the facts over, and the same task becomes claimable without
// the user clicking again.
//
// The refusals are ordered by what the user can do about them: a material outside
// the actor's scope is not theirs to see (the 404/403 from `GetMaterial`), and a
// visible material with no prepared video and no source to prepare from is one
// nothing can be done about. Answering 409 for that rather than 500 is the whole
// point of asking here.
//
// The download belongs to the user, not to a machine: any Desktop the actor logs
// in on can claim it, and nothing is assigned here.
func (s *Service) CreateDownload(ctx context.Context, actor identityservice.PublicUser, materialID int64) (transferdto.Task, error) {
	material, err := s.getMaterial(actor, materialID)
	if err != nil {
		return transferdto.Task{}, err
	}
	// Downloading the original video is a personal action from "My Materials",
	// not a library action (§5.3.4). A visible material alone is not enough:
	// the current user's relation must be in use, so a removed relation or a
	// direct API call without one fails closed. The Cloud worker's preparation
	// path does not pass through here and keeps working without a user.
	inUse, err := s.store.HasActiveUsage(material.ID, actor.ID)
	if err != nil {
		return transferdto.Task{}, fmt.Errorf("check material %d usage for user %d: %w", material.ID, actor.ID, err)
	}
	if !inUse {
		return transferdto.Task{}, ErrUsageForbidden
	}
	// `ready` is a projection, so a ready row still has to prove its object is
	// in the bucket before this route promises a download: the incident this
	// branch was added for is exactly a row that said ready about an object that
	// no longer existed, and the Local Agent would have received a 404 grant.
	var objectKey, sha256 string
	var size int64
	needsPreparation := material.VideoStatus != model.VideoReady
	if !needsPreparation {
		if objectKey, size, sha256, err = videoFacts(material); err != nil {
			return transferdto.Task{}, err
		}
		exists, existsErr := s.links.ObjectExists(ctx, objectKey)
		if existsErr != nil {
			return transferdto.Task{}, fmt.Errorf("check material %d source object: %w", material.ID, existsErr)
		}
		if !exists {
			if strings.TrimSpace(material.SourceURL) == "" {
				// Ready, object gone, and no page address to re-resolve: nothing can
				// make this download possible, and the 409 is honest.
				return transferdto.Task{}, ErrMaterialUnavailable
			}
			needsPreparation = true
			objectKey, size, sha256 = "", 0, ""
		}
	} else if strings.TrimSpace(material.SourceURL) == "" {
		// Not ready *and* nothing to prepare from: the material's source address is
		// gone, so waiting produces no file and the 409 is honest. Every other
		// not-ready state is a wait rather than a refusal, which is why the
		// condition is this narrow rather than `!ready`.
		return transferdto.Task{}, ErrMaterialUnavailable
	}
	input := transferservice.CreateUserDownloadInput{
		TeamID:          material.TeamID,
		AssetID:         material.ID,
		AssetTitle:      material.Title,
		GameName:        s.gameNameOf(material),
		PublishedAt:     material.PublishedAt,
		SourceObjectKey: objectKey,
		RequestedBy:     actor.ID,
		TotalBytes:      size,
		ExpectedSHA256:  sha256,
	}
	if needsPreparation {
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
		// that does not exist. A ready row whose object is gone needs the
		// key-and-timestamp compare (`MarkVideoRepreparing`) so a preparation that
		// finished in the meantime is not taken back; every other state uses the
		// ordinary marker.
		var preparing bool
		if material.VideoStatus == model.VideoReady {
			preparing, err = s.store.MarkVideoRepreparing(material.TeamID, material.ID, material.SourceObjectKey, material.VideoPreparedAt, s.now().UTC())
		} else {
			preparing, err = s.store.MarkVideoPreparing(material.TeamID, material.ID, s.now().UTC())
		}
		if err != nil {
			return transferdto.Task{}, fmt.Errorf("mark material %d preparing: %w", material.ID, err)
		}
		if !preparing {
			// The guard refused, so the material moved between the read at the top of
			// this method and the statement above. There are two ways that happens and
			// they need different tasks: another preparation is now running (waiting is
			// exactly the plan), or one finished and the row is `ready` again, in which
			// case a wait would hold a file back that can be fetched now. Only a fresh
			// read tells the two apart, and guessing wrong on the second strands a
			// download.
			refreshed, readErr := s.getMaterial(actor, materialID)
			if readErr != nil {
				return transferdto.Task{}, readErr
			}
			if refreshed.VideoStatus == model.VideoReady {
				key, freshSize, freshSHA, factsErr := videoFacts(refreshed)
				if factsErr != nil {
					return transferdto.Task{}, factsErr
				}
				exists, existsErr := s.links.ObjectExists(ctx, key)
				if existsErr != nil {
					return transferdto.Task{}, fmt.Errorf("check material %d source object: %w", refreshed.ID, existsErr)
				}
				if !exists {
					// The projection says ready again but the object is still missing;
					// queuing a download would only hand the agent a 404.
					return transferdto.Task{}, ErrMaterialUnavailable
				}
				input.DependencyTaskID = ""
				input.SourceObjectKey, input.TotalBytes, input.ExpectedSHA256 = key, freshSize, freshSHA
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
	if input.DependencyTaskID != "" {
		// The preparation may have finished between `Ensure` and the insert, in which
		// case the live hand-over saw no row to update. The attachment is best-effort:
		// a failure here leaves the row for the scheduled sweep, and the task body the
		// client already holds is still the truth until the sweep releases it.
		if attached, ok, attachErr := s.transfers.AttachCompletedDependency(task.ID); attachErr == nil && ok {
			return attached, nil
		}
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

// MarkVideoNotPrepared takes back the `downloading` projection a cancelled
// preparation left behind.
//
// It is the third of the worker's writes and the one that closes a state nothing
// else could leave: the material says a preparation is in flight, the task that
// would have delivered one is terminal and un-leasable, and no retry will pick it
// up. The user's own remedy is another click, and that click cannot use the
// projection at all — `MarkVideoPreparing` refuses `downloading` — so the row would
// read 「准备中」 for a preparation that ended. See `repository.MarkVideoNotPrepared`
// for why the predicate makes that safe for a material that did get prepared, and
// why a write that matched nothing is not an error.
func (s *Service) MarkVideoNotPrepared(teamID identityservice.TeamID, materialID int64) error {
	_, err := s.store.MarkVideoNotPrepared(teamID, materialID, s.now().UTC())
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

// gameNameOf names the game a download should be filed under, or "" when there
// is none to name. Naming is cosmetic: a lookup that fails (no game assigned, an
// unknown id, a store error) degrades to empty and the executor falls back to a
// fixed placeholder rather than blocking the download on it.
func (s *Service) gameNameOf(material model.Material) string {
	if material.GameID == nil {
		return ""
	}
	name, ok, err := s.resolveGame(*material.GameID)
	if err != nil || !ok {
		return ""
	}
	return name
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

// canAccessMaterial is the team boundary, not a game boundary: the library is
// the team's public pool (§5.3.3), and a material's game is only a filter. An
// admin reads across teams and may narrow the list by target team. A material
// without a game is visible too — it is still the team's material.
func canAccessMaterial(actor identityservice.PublicUser, material model.Material) bool {
	if actor.Status != identityservice.UserStatusEnabled {
		return false
	}
	if actor.Role == identityservice.RoleAdmin {
		return true
	}
	return actor.TeamID != nil && *actor.TeamID == material.TeamID
}
