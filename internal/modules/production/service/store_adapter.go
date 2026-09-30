package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	runtimeservice "github.com/wt-media/wt-media-cloud/internal/modules/runtimebinding/service"
)

type mysqlStore struct{}

func (mysqlStore) FindMaterial(id int64) (model.Material, bool, error) {
	return repository.FindMaterialUnscoped(id)
}

func (mysqlStore) ListMaterials(filter repository.MaterialFilter) ([]model.Material, error) {
	return repository.ListMaterials(filter)
}

func (mysqlStore) CreateOrRestoreUsage(input repository.CreateUsageInput, now time.Time) (model.MaterialUsage, bool, error) {
	return repository.CreateOrRestoreUsage(input, now)
}

func (mysqlStore) ListUsages(userID identityservice.UserID) ([]model.MaterialUsage, error) {
	return repository.ListUsages(userID)
}

func (mysqlStore) FindUsageForUser(usageID int64, userID identityservice.UserID) (model.MaterialUsage, bool, error) {
	return repository.FindUsageForUser(usageID, userID)
}

func (mysqlStore) RemoveUsageByID(usageID int64, userID identityservice.UserID, now time.Time) (bool, error) {
	return repository.RemoveUsageByID(usageID, userID, now)
}
func (mysqlStore) RestoreUsageByID(usageID int64, userID identityservice.UserID, now time.Time) error {
	return repository.RestoreUsageByID(usageID, userID, now)
}

func (mysqlStore) MarkVideoPreparing(teamID identityservice.TeamID, materialID int64, now time.Time) (bool, error) {
	return repository.MarkVideoPreparing(teamID, materialID, now)
}

func (mysqlStore) MarkVideoReady(teamID identityservice.TeamID, materialID int64, facts repository.VideoFacts, now time.Time) (bool, error) {
	return repository.MarkVideoReady(teamID, materialID, facts, now)
}

func (mysqlStore) MarkVideoFailed(teamID identityservice.TeamID, materialID int64, message string, now time.Time) (bool, error) {
	return repository.MarkVideoFailed(teamID, materialID, message, now)
}

func (mysqlStore) MarkVideoNotPrepared(teamID identityservice.TeamID, materialID int64, now time.Time) (bool, error) {
	return repository.MarkVideoNotPrepared(teamID, materialID, now)
}

// runtimeNodeResolver is the whole of this module's dependency on the
// runtime-binding domain, and productionTransferCreator the whole of its
// dependency on the transfer domain. Both are types rather than calls inside the
// service so that a test answers them without a node table or a task table, and
// so that the two module boundaries this module crosses are visible in one file.
type runtimeNodeResolver struct{}

func (runtimeNodeResolver) ResolveTrustedLocalNode(userID identityservice.UserID) (runtimeservice.AgentNode, error) {
	return runtimeservice.ResolveTrustedLocalNode(userID)
}

type productionTransferCreator struct{}

func (productionTransferCreator) CreateUserDownload(input transferservice.CreateUserDownloadInput) (transferdto.Task, error) {
	return transferservice.CreateUserDownload(input)
}

func (productionTransferCreator) EnsureMaterialSourcePrepare(input transferservice.EnsureMaterialSourcePrepareInput) (transferdto.Task, error) {
	return transferservice.EnsureMaterialSourcePrepare(input)
}

// productionObjectLinker is this module's whole dependency on the storage
// boundary; the composition itself lives in `infra/storage`.
type productionObjectLinker struct{}

func (productionObjectLinker) PublicObjectURL(key string) (string, error) {
	return storage.PublicURL(key)
}
