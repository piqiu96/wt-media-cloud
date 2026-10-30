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
func (mysqlStore) Update(record model.AccountRecord, replaceCategoryIDs *[]identityservice.CategoryID) error {
	return repository.Update(record, replaceCategoryIDs)
}
func (mysqlStore) List(query dto.AccountFilter) ([]model.AccountRecord, error) {
	return repository.List(repository.AccountFilter{
		UserID:              query.UserID,
		CategoryIDs:         query.CategoryIDs,
		Platform:            query.Platform,
		Enabled:             query.Enabled,
		PlatformLoginStatus: query.PlatformLoginStatus,
		Search:              query.Search,
		ProfileSearch:       query.ProfileSearch,
		AnyTags:             query.AnyTags,
		AllTags:             query.AllTags,
		ExcludeTags:         query.ExcludeTags,
	})
}
func (mysqlStore) AddTags(userID identityservice.UserID, accountIDs, tags []string, createdAt time.Time) error {
	return repository.AddTags(userID, accountIDs, tags, createdAt)
}
func (mysqlStore) RemoveTags(userID identityservice.UserID, accountIDs, tags []string) error {
	return repository.RemoveTags(userID, accountIDs, tags)
}
func (mysqlStore) ListTags(accountIDs []string) (map[string][]string, error) {
	return repository.ListTags(accountIDs)
}
func (mysqlStore) AppendAudit(event identityservice.AuditEvent) error {
	return repository.AppendAudit(event)
}
func (mysqlStore) CreateGroup(group model.AccountGroup) (string, error) {
	return repository.CreateGroup(group)
}
func (mysqlStore) FindGroup(id string) (model.AccountGroup, bool, error) {
	return repository.FindGroup(id)
}
func (mysqlStore) ListGroups(userID identityservice.UserID) ([]model.AccountGroup, error) {
	return repository.ListGroups(userID)
}
func (mysqlStore) UpdateGroup(group model.AccountGroup) error { return repository.UpdateGroup(group) }
func (mysqlStore) DeleteGroup(id string) error                { return repository.DeleteGroup(id) }

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
