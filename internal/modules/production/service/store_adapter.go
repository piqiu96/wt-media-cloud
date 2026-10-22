package service

import (
	"context"
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	transferdto "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/dto"
	transferservice "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/service"
	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
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

func (mysqlStore) FindUsage(usageID int64) (model.MaterialUsage, bool, error) {
	return repository.FindUsage(usageID)
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

func (mysqlStore) MarkVideoRepreparing(teamID identityservice.TeamID, materialID int64, expectedObjectKey string, expectedPreparedAt *time.Time, now time.Time) (bool, error) {
	return repository.MarkVideoRepreparing(teamID, materialID, expectedObjectKey, expectedPreparedAt, now)
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

// productionTransferCreator is the whole of this module's dependency on the
// transfer domain. It is a type rather than a call inside the service so that a
// test answers it without a task table, and so that the module boundary is
// visible in one file.
type productionTransferCreator struct{}

func (productionTransferCreator) CreateUserDownload(input transferservice.CreateUserDownloadInput) (transferdto.Task, error) {
	return transferservice.CreateUserDownload(input)
}

func (productionTransferCreator) EnsureMaterialSourcePrepare(input transferservice.EnsureMaterialSourcePrepareInput) (transferdto.Task, error) {
	return transferservice.EnsureMaterialSourcePrepare(input)
}

func (productionTransferCreator) LatestUserDownloadStatuses(userID identityservice.UserID, materialIDs []int64) (map[int64]string, error) {
	return transferservice.LatestUserDownloadStatuses(userID, materialIDs)
}

func (productionTransferCreator) AttachCompletedDependency(taskID string) (transferdto.Task, bool, error) {
	return transferservice.AttachCompletedDependency(taskID)
}

// productionObjectLinker is this module's whole dependency on the storage
// boundary; the minting itself lives in `infra/storage`.
type productionObjectLinker struct{}

// PresignObjectURL mints the grant with the lifetime configuration already
// decides and publishes only its address. The expiry stops here for the same
// reason it is absent from the contract body: nothing on the wire reads it.
func (productionObjectLinker) PresignObjectURL(ctx context.Context, key string) (string, error) {
	grant, err := storage.PresignGet(ctx, key)
	if err != nil {
		return "", err
	}
	return grant.URL, nil
}

// ObjectExists is a metadata read, not a download: `Stat` costs one HEAD and no
// bytes, so asking before every download and every preview does not move the
// data the operation was going to move anyway.
func (productionObjectLinker) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := storage.Get().Stat(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
