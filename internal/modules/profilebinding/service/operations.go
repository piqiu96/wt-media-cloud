package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/repository"
)

func SubmitScan(actor service.PublicUser, input dto.SnapshotInput) (model.ProfileScan, error) {
	return newService(mysqlStore{}).SubmitScan(actor, input)
}

func GetScan(actor service.PublicUser, scanID string) (model.ProfileScan, error) {
	return newService(mysqlStore{}).GetScan(actor, scanID)
}

func ConfirmScan(actor service.PublicUser, scanID string) (model.ProfileScan, error) {
	return newService(mysqlStore{}).ConfirmScan(actor, scanID)
}

func ConfirmMainIdentity(actor service.PublicUser, scanID string) (model.ProfileScan, error) {
	return newService(mysqlStore{}).ConfirmMainIdentity(actor, scanID)
}

func ConfirmMainIdentityDirect(actor service.PublicUser, input dto.MainIdentityInput) (model.BitAccountBinding, error) {
	return newService(mysqlStore{}).ConfirmMainIdentityDirect(actor, input)
}

func ClearMainIdentity(actor service.PublicUser, userID service.UserID) error {
	return newService(mysqlStore{}).ClearMainIdentity(actor, userID)
}

func RejectScan(actor service.PublicUser, scanID string) error {
	return newService(mysqlStore{}).RejectScan(actor, scanID)
}

func ListProfiles(actor service.PublicUser, userID service.UserID) ([]model.BrowserProfile, error) {
	return newService(mysqlStore{}).ListProfiles(actor, userID)
}

func DeleteProfile(actor service.PublicUser, profileID string) error {
	return newService(mysqlStore{}).DeleteProfile(actor, profileID)
}

func UpdateProfile(actor service.PublicUser, profileID string, cloudRemark *string, businessStatus *model.ProfileBusinessStatus) (model.BrowserProfile, error) {
	return newService(mysqlStore{}).UpdateProfile(actor, profileID, cloudRemark, businessStatus)
}

func AssignProfileOwner(actor service.PublicUser, profileID string, target service.PublicUser) (model.BrowserProfile, error) {
	return newService(mysqlStore{}).AssignProfileOwner(actor, profileID, target)
}

func GetActiveProfile(actor service.PublicUser, profileID string) (model.BrowserProfile, error) {
	return newService(mysqlStore{}).GetActiveProfile(actor, profileID)
}

func ResolveProfile(profileID string) (service.UserID, bool, bool, error) {
	return repository.ResolveProfile(profileID)
}

func ResolveProfileForAccountCheck(profileID string) (string, service.UserID, string, bool, bool, error) {
	return repository.ResolveProfileForAccountCheck(profileID)
}

func ResolveProxyForAccountCheck(profileID string) (string, string, string, *time.Time, bool, error) {
	return repository.ResolveProxyForAccountCheck(profileID)
}

func BindProxy(profileID, proxyID, proxyType, proxyHost string, proxyPort int) (model.BrowserProfile, error) {
	return repository.BindProxy(profileID, proxyID, proxyType, proxyHost, proxyPort)
}

func UnbindProxy(profileID, expectedProxyID string) (model.BrowserProfile, error) {
	return repository.UnbindProxy(profileID, expectedProxyID)
}

func CountProfilesByProxyID(proxyID string) (int, error) {
	return repository.CountProfilesByProxyID(proxyID)
}

func GetProfile(profileID string) (model.BrowserProfile, bool, error) {
	return repository.GetProfile(profileID)
}
