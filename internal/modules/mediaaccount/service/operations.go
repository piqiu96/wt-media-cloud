package service

import identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"

// Package-level operations preserve the existing Media Account behavior.
func CreateAccount(actor identityservice.PublicUser, input CreateAccountInput) (Account, error) {
	return defaultAccountService().CreateAccount(actor, input)
}

func GetAccountRecord(actor identityservice.PublicUser, accountID string) (AccountRecord, error) {
	return defaultAccountService().GetAccountRecord(actor, accountID)
}

func GetOwnedAccountRecord(actor identityservice.PublicUser, accountID string) (AccountRecord, error) {
	return defaultAccountService().GetOwnedAccountRecord(actor, accountID)
}

func GetAccount(actor identityservice.PublicUser, accountID string) (Account, error) {
	return defaultAccountService().GetAccount(actor, accountID)
}

func ListAccounts(actor identityservice.PublicUser, filter AccountFilter) ([]Account, error) {
	return defaultAccountService().ListAccounts(actor, filter)
}

func UpdateAccount(actor identityservice.PublicUser, accountID string, input UpdateAccountInput) (Account, error) {
	return defaultAccountService().UpdateAccount(actor, accountID, input)
}

func IdentifyAccount(actor identityservice.PublicUser, accountID string, input IdentifyAccountInput) (Account, error) {
	return defaultAccountService().IdentifyAccount(actor, accountID, input)
}

func StartLocalAccountCheck(actor identityservice.PublicUser, accountID string, input AccountCheckStartInput) (AccountCheckStart, error) {
	return defaultAccountService().StartLocalAccountCheck(actor, accountID, input)
}

func StartCookieRead(actor identityservice.PublicUser, accountID string, input CookieReadStartInput) (CookieReadStart, error) {
	return defaultAccountService().StartCookieRead(actor, accountID, input)
}

func ApplyCookieReadResult(actor identityservice.PublicUser, accountID string, input CookieReadResultInput) (Account, error) {
	return defaultAccountService().ApplyCookieReadResult(actor, accountID, input)
}

func ApplyLocalAccountCheckResult(actor identityservice.PublicUser, accountID string, input AccountCheckResultInput) (Account, error) {
	return defaultAccountService().ApplyLocalAccountCheckResult(actor, accountID, input)
}

func AddTags(actor identityservice.PublicUser, accountIDs, tags []string) error {
	return defaultAccountService().AddTags(actor, accountIDs, tags)
}

func RemoveTags(actor identityservice.PublicUser, accountIDs, tags []string) error {
	return defaultAccountService().RemoveTags(actor, accountIDs, tags)
}

func BindProfile(actor identityservice.PublicUser, accountID, profileID string) (Account, error) {
	return defaultAccountService().BindProfile(actor, accountID, profileID)
}

func UnbindProfile(actor identityservice.PublicUser, accountID string) (Account, error) {
	return defaultAccountService().UnbindProfile(actor, accountID)
}

func CreateAccountGroup(actor identityservice.PublicUser, input CreateAccountGroupInput) (AccountGroup, error) {
	return defaultAccountService().CreateAccountGroup(actor, input)
}

func ListAccountGroups(actor identityservice.PublicUser) ([]AccountGroup, error) {
	return defaultAccountService().ListAccountGroups(actor)
}

func UpdateAccountGroup(actor identityservice.PublicUser, groupID string, input UpdateAccountGroupInput) (AccountGroup, error) {
	return defaultAccountService().UpdateAccountGroup(actor, groupID, input)
}

func DeleteAccountGroup(actor identityservice.PublicUser, groupID string) error {
	return defaultAccountService().DeleteAccountGroup(actor, groupID)
}

func ListAccountsByGroup(actor identityservice.PublicUser, groupID string) ([]Account, error) {
	return defaultAccountService().ListAccountsByGroup(actor, groupID)
}

func StartAccountCheck(actor identityservice.PublicUser, accountID string, input AccountCheckStartInput) (AccountCheckStart, error) {
	return defaultAccountService().StartLocalAccountCheck(actor, accountID, input)
}

func ApplyAccountCheckResult(actor identityservice.PublicUser, accountID string, input AccountCheckResultInput) (Account, error) {
	return defaultAccountService().ApplyLocalAccountCheckResult(actor, accountID, input)
}
