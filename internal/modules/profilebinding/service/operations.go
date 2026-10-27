package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/repository"
)

func SubmitScan(actor service.PublicUser, input dto.SnapshotInput) (model.ProfileScan, error) {
	return newService(mysqlStore{}, WithObservedFreshFor(processObservedFreshFor)).SubmitScan(actor, input)
}

func GetScan(actor service.PublicUser, scanID string) (model.ProfileScan, error) {
	return newService(mysqlStore{}).GetScan(actor, scanID)
}

func ConfirmScan(actor service.PublicUser, scanID string, adoptBitProfileIDs []string) (model.ProfileScan, error) {
	return newService(mysqlStore{}).ConfirmScan(actor, scanID, adoptBitProfileIDs)
}

func GetBitAccount(userID service.UserID) (model.BitAccountBinding, bool, error) {
	return newService(mysqlStore{}).GetBitAccount(userID)
}

func BindBitAccount(userID service.UserID, input dto.BitAccountBindInput) (model.BitAccountBinding, error) {
	return newService(mysqlStore{}, WithObservedFreshFor(processObservedFreshFor)).BindBitAccount(userID, input)
}

func RejectScan(actor service.PublicUser, scanID string) error {
	return newService(mysqlStore{}).RejectScan(actor, scanID)
}

func ListProfiles(actor service.PublicUser, userID service.UserID, filter ProfileListFilter) ([]model.BrowserProfile, error) {
	return newService(mysqlStore{}).ListProfiles(actor, userID, filter)
}

func DeleteProfile(actor service.PublicUser, profileID string) error {
	return newService(mysqlStore{}).DeleteProfile(actor, profileID)
}

func UpdateProfile(actor service.PublicUser, profileID string, cloudRemark *string, businessStatus *model.ProfileBusinessStatus) (model.BrowserProfile, error) {
	return newService(mysqlStore{}).UpdateProfile(actor, profileID, cloudRemark, businessStatus)
}

func GetActiveProfile(actor service.PublicUser, profileID string) (model.BrowserProfile, error) {
	return newService(mysqlStore{}).GetActiveProfile(actor, profileID)
}

func ResolveProfile(profileID string) (service.UserID, bool, bool, bool, error) {
	return repository.ResolveProfile(profileID)
}

func ResolveProfileForAccountCheck(profileID string) (string, service.UserID, string, bool, bool, bool, error) {
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

func ReleaseProxy(profileID, expectedProxyID string) (model.BrowserProfile, error) {
	return repository.ReleaseProxy(profileID, expectedProxyID)
}

func ClearObservedProxy(profileID string) (model.BrowserProfile, error) {
	return repository.ClearObservedProxy(profileID)
}

func CountProfilesByProxyID(proxyID string) (int, error) {
	return repository.CountProfilesByProxyID(proxyID)
}

func GetProfile(profileID string) (model.BrowserProfile, bool, error) {
	return repository.GetProfile(profileID)
}
