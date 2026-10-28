package service

import (
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/repository"
)

type mysqlStore struct{}

func (mysqlStore) FindBinding(userID identityservice.UserID) (model.BitAccountBinding, bool, error) {
	return repository.FindBinding(userID)
}
func (mysqlStore) ListProfiles(userID identityservice.UserID, filter ProfileListFilter) ([]model.BrowserProfile, error) {
	return repository.ListProfiles(repository.ProfileListFilter{
		UserID:       userID,
		ID:           filter.ID,
		BitName:      filter.BitName,
		BitGroup:     filter.BitGroup,
		BitProfileID: filter.BitProfileID,
		Remark:       filter.Remark,
		LocalState:   filter.LocalState,
		Enabled:      filter.Enabled,
	})
}
func (mysqlStore) BindBitAccount(userID identityservice.UserID, mainUserID string, rebind bool, at time.Time) (model.BitAccountBinding, error) {
	return repository.BindBitAccount(userID, mainUserID, rebind, at)
}
func (mysqlStore) GetProfile(profileID string) (model.BrowserProfile, bool, error) {
	return repository.GetProfile(profileID)
}
func (mysqlStore) GetProfileInScope(profileID string) (model.BrowserProfile, bool, error) {
	return repository.GetProfileInScope(profileID)
}
func (mysqlStore) CreateScan(scan model.ProfileScan) error { return repository.CreateScan(scan) }
func (mysqlStore) FindScan(scanID string) (model.ProfileScan, bool, error) {
	return repository.FindScan(scanID)
}
func (mysqlStore) FindScanByUser(userID identityservice.UserID) (model.ProfileScan, bool, error) {
	return repository.FindScanByUser(userID)
}
func (mysqlStore) DeleteScanByUser(userID identityservice.UserID) error {
	return repository.DeleteScanByUser(userID)
}
func (mysqlStore) ApplyScan(scan model.ProfileScan, observedBitProfileIDs []string, at time.Time) error {
	return repository.ApplyScan(scan, observedBitProfileIDs, at)
}
func (mysqlStore) DeleteProfile(id string) error { return repository.DeleteProfile(id) }
func (mysqlStore) UpdateProfile(profileID string, remark *string, enabled *bool, at time.Time) error {
	return repository.UpdateProfile(profileID, remark, enabled, at)
}
func (mysqlStore) ProfileHasAccountReferences(profileID string) (bool, error) {
	return repository.ProfileHasAccountReferences(profileID)
}
func (mysqlStore) ProfileHasDependencies(profileID string) (bool, error) {
	return repository.ProfileHasDependencies(profileID)
}
func (mysqlStore) ListProfileOwners(mainUserID string) (map[string]identityservice.UserID, error) {
	return repository.ListProfileOwners(mainUserID)
}
