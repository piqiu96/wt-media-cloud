package service

import (
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/repository"
	profilebindingservice "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/service"
	profileguardservice "github.com/wt-media/wt-media-cloud/internal/modules/profileguard/service"
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
func (mysqlStore) Update(record model.AccountRecord, replaceGameIDs *[]string) error {
	return repository.Update(record, replaceGameIDs)
}
func (mysqlStore) List(query dto.AccountFilter) ([]model.AccountRecord, error) {
	return repository.List(query)
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

func (productionProfileResolver) ResolveProfile(profileID string) (identityservice.UserID, bool, bool, error) {
	return profilebindingservice.ResolveProfile(profileID)
}

type productionProfileFactResolver struct{}

func (productionProfileFactResolver) ResolveProfileForAccountCheck(profileID string) (string, identityservice.UserID, string, bool, bool, error) {
	return profilebindingservice.ResolveProfileForAccountCheck(profileID)
}
func (productionProfileFactResolver) ResolveProxyForAccountCheck(profileID string) (string, string, string, *time.Time, bool, error) {
	return profilebindingservice.ResolveProxyForAccountCheck(profileID)
}

type productionSensitiveTaskCreator struct{}

func (productionSensitiveTaskCreator) CreateAuthorizedTask(task profileguardservice.SensitiveTask) error {
	return profileguardservice.CreateAuthorizedTask(task)
}

type productionUserResolver struct{}

func (productionUserResolver) ResolveUser(userID identityservice.UserID) (identityservice.PublicUser, bool, error) {
	return identityservice.ResolveUser(userID)
}

type productionGameResolver struct{}

func (productionGameResolver) ResolveGame(gameID string) (identityservice.OperationGame, bool, error) {
	return identityservice.ResolveGame(gameID)
}

func defaultAccountService() *accountService {
	return newAccountService(
		mysqlStore{},
		withProfileResolver(productionProfileResolver{}),
		withProfileFactResolver(productionProfileFactResolver{}),
		withSensitiveTaskCreator(productionSensitiveTaskCreator{}),
		withUserResolver(productionUserResolver{}),
		withGameResolver(productionGameResolver{}),
	)
}
