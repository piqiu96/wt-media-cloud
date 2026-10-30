package service

import (
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/repository"
	profilebindingservice "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
)

type mysqlStore struct{}

func (mysqlStore) ListPage(query dto.AccountFilter) ([]model.AccountRecord, model.AccountCounts, error) {
	return repository.ListPage(repository.AccountFilter(query))
}

func (mysqlStore) UpdateMetadata(record model.AccountRecord, input dto.UpdateAccountInput) error {
	return repository.UpdateMetadata(record, input)
}
func (mysqlStore) SetProfile(record model.AccountRecord) error { return repository.SetProfile(record) }

func (mysqlStore) StartCheck(record model.AccountRecord) error { return repository.StartCheck(record) }
func (mysqlStore) CompleteCheck(record model.AccountRecord) error {
	return repository.CompleteCheck(record)
}

func (mysqlStore) Create(record model.AccountRecord) (string, error) {
	return repository.Create(record)
}
func (mysqlStore) Find(id string) (model.AccountRecord, bool, error) { return repository.Find(id) }
func (mysqlStore) FindByIdentity(userID identityservice.UserID, platform model.Platform, platformAccountID string) (model.AccountRecord, bool, error) {
	return repository.FindByIdentity(userID, platform, platformAccountID)
}
func (mysqlStore) FindByProfilePlatform(profileID string, platform model.Platform) (model.AccountRecord, bool, error) {
	return repository.FindByProfilePlatform(profileID, platform)
}
func (mysqlStore) List(query dto.AccountFilter) ([]model.AccountRecord, error) {
	return repository.List(repository.AccountFilter(query))
}
func (mysqlStore) ListTags(accountIDs []string) (map[string][]string, error) {
	return repository.ListTags(accountIDs)
}
func (mysqlStore) AppendAudit(event identityservice.AuditEvent) error {
	return repository.AppendAudit(event)
}

type productionProfileResolver struct{}

func (productionProfileResolver) ResolveProfile(profileID string) (identityservice.UserID, bool, bool, bool, error) {
	return profilebindingservice.ResolveProfile(profileID)
}

type productionProfileFactResolver struct{}

func (productionProfileFactResolver) ResolveProfileForAccountCheck(profileID string) (string, identityservice.UserID, string, bool, bool, bool, error) {
	return profilebindingservice.ResolveProfileForAccountCheck(profileID)
}
func (productionProfileFactResolver) ResolveProxyForAccountCheck(profileID string) (string, bool, string, *time.Time, bool, error) {
	return profilebindingservice.ResolveProxyForAccountCheck(profileID)
}

type productionUserResolver struct{}

func (productionUserResolver) ResolveUser(userID identityservice.UserID) (identityservice.PublicUser, bool, error) {
	return identityservice.ResolveUser(userID)
}

type productionCategoryResolver struct{}

func (productionCategoryResolver) ResolveCategory(categoryID identityservice.CategoryID) (identityservice.Category, bool, error) {
	return identityservice.ResolveCategory(categoryID)
}

func defaultAccountService() *accountService {
	return newAccountService(
		mysqlStore{},
		withProfileResolver(productionProfileResolver{}),
		withProfileFactResolver(productionProfileFactResolver{}),
		withUserResolver(productionUserResolver{}),
		withCategoryResolver(productionCategoryResolver{}),
	)
}

func (mysqlStore) MutateCookie(id string, userID identityservice.UserID, mutate func(*model.AccountRecord, *string) error) (model.AccountRecord, error) {
	return repository.MutateCookie(id, userID, mutate)
}
