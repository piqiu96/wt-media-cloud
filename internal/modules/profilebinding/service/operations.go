package service

import (
	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
)

func SubmitScan(actor service.PublicUser, input dto.SnapshotInput) (model.ProfileScan, error) {
	return NewService(mysqlStore{}).SubmitScan(actor, input)
}

func GetScan(actor service.PublicUser, scanID string) (model.ProfileScan, error) {
	return NewService(mysqlStore{}).GetScan(actor, scanID)
}

func ConfirmScan(actor service.PublicUser, scanID string) (model.ProfileScan, error) {
	return NewService(mysqlStore{}).ConfirmScan(actor, scanID)
}

func ConfirmMainIdentity(actor service.PublicUser, scanID string) (model.ProfileScan, error) {
	return NewService(mysqlStore{}).ConfirmMainIdentity(actor, scanID)
}

func ConfirmMainIdentityDirect(actor service.PublicUser, input dto.MainIdentityInput) (model.BitAccountBinding, error) {
	return NewService(mysqlStore{}).ConfirmMainIdentityDirect(actor, input)
}

func ClearMainIdentity(actor service.PublicUser, userID service.UserID) error {
	return NewService(mysqlStore{}).ClearMainIdentity(actor, userID)
}

func RejectScan(actor service.PublicUser, scanID string) error {
	return NewService(mysqlStore{}).RejectScan(actor, scanID)
}

func ListProfiles(actor service.PublicUser, userID service.UserID) ([]model.BrowserProfile, error) {
	return NewService(mysqlStore{}).ListProfiles(actor, userID)
}

func DeleteProfile(actor service.PublicUser, profileID string) error {
	return NewService(mysqlStore{}).DeleteProfile(actor, profileID)
}

func UpdateProfile(actor service.PublicUser, profileID string, cloudRemark *string, businessStatus *model.ProfileBusinessStatus) (model.BrowserProfile, error) {
	return NewService(mysqlStore{}).UpdateProfile(actor, profileID, cloudRemark, businessStatus)
}

func AssignProfileOwner(actor service.PublicUser, profileID string, target service.PublicUser) (model.BrowserProfile, error) {
	return NewService(mysqlStore{}).AssignProfileOwner(actor, profileID, target)
}

func GetActiveProfile(actor service.PublicUser, profileID string) (model.BrowserProfile, error) {
	return NewService(mysqlStore{}).GetActiveProfile(actor, profileID)
}
