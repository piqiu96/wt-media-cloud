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
		UserID:         userID,
		ID:             filter.ID,
		Name:           filter.Name,
		Group:          filter.Group,
		BitProfileID:   filter.BitProfileID,
		Remark:         filter.Remark,
		LocalStatus:    filter.LocalStatus,
		BusinessStatus: filter.BusinessStatus,
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
func (mysqlStore) ApplyScan(scan model.ProfileScan, at time.Time) error {
	return repository.ApplyScan(scan, at)
}
func (mysqlStore) DeleteProfile(id string) error { return repository.DeleteProfile(id) }
func (mysqlStore) UpdateProfile(profileID string, cloudRemark *string, businessStatus *model.ProfileBusinessStatus, at time.Time) error {
	return repository.UpdateProfile(profileID, cloudRemark, businessStatus, at)
}
func (mysqlStore) ProfileHasAccountReferences(profileID string) (bool, error) {
	return repository.ProfileHasAccountReferences(profileID)
}
func (mysqlStore) ProfileHasDependencies(profileID string) (bool, error) {
	return repository.ProfileHasDependencies(profileID)
}
func (mysqlStore) AssignProfileOwner(profileID string, userID identityservice.UserID, teamID *identityservice.TeamID, actorID identityservice.UserID, at time.Time) error {
	return repository.AssignProfileOwner(profileID, userID, teamID, actorID, at)
}
