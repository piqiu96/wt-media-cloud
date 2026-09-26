// Package service owns material visibility and the “My Materials” use case.
package service

import (
	"errors"
	"fmt"
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

// CreateDownload queues one user download of a prepared material, for the actor's
// own freshest Local Agent node.
//
// The three refusals are ordered by what the user can do about them: a material
// outside the actor's scope is not theirs to see (the 404/403 from `GetMaterial`),
// a visible material with no prepared video is one they can wait for, and no
// fresh node is one they can fix by opening the app on a machine. Answering 409
// for the last two rather than 500 is the whole point of asking here.
//
// The download is **never** queued without a node. An executor claims on its own
// credential, so a task with no assignee would be claimed by whichever device
// polled first — a file the user asked for on one machine landing on another.
func (s *Service) CreateDownload(actor identityservice.PublicUser, materialID int64) (transferdto.Task, error) {
	material, err := s.GetMaterial(actor, materialID)
	if err != nil {
		return transferdto.Task{}, err
	}
	if material.VideoStatus != model.VideoReady {
		return transferdto.Task{}, ErrMaterialUnavailable
	}
	if material.SourceObjectKey == "" || material.VideoSizeBytes == nil || *material.VideoSizeBytes <= 0 || material.VideoSHA256 == "" {
		// `ready` is the projection's promise that all four facts were written in
		// the same step. A ready row missing one of them is this module's own
		// inconsistency, so it is reported as a fault rather than as the 409 the
		// user would read as "try again later".
		return transferdto.Task{}, fmt.Errorf("material %d is ready but its video facts are incomplete", material.ID)
	}
	node, err := s.nodes.ResolveFreshLocalNode(actor.ID)
	if err != nil {
		// Everything the resolver refuses means the same thing here, including a
		// store failure: from this route's side there is no node to download to.
		// The distinction is kept in the log, not in the response.
		return transferdto.Task{}, ErrLocalNodeUnavailable
	}
	task, err := s.transfers.CreateUserDownload(transferservice.CreateUserDownloadInput{
		TeamID:          material.TeamID,
		AssetID:         material.ID,
		AssetTitle:      material.Title,
		SourceObjectKey: material.SourceObjectKey,
		RequestedBy:     actor.ID,
		AssignedNodeID:  node.ID,
		TotalBytes:      *material.VideoSizeBytes,
		ExpectedSHA256:  material.VideoSHA256,
	})
	if err != nil {
		if errors.Is(err, transferservice.ErrInvalidInput) {
			return transferdto.Task{}, ErrInvalidInput
		}
		return transferdto.Task{}, fmt.Errorf("queue user download for material %d: %w", material.ID, err)
	}
	return task, nil
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
