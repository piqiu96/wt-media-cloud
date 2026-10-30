package service

import "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
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

func BindProfile(actor identityservice.PublicUser, accountID, profileID string) (Account, error) {
	return defaultAccountService().BindProfile(actor, accountID, profileID)
}

func UnbindProfile(actor identityservice.PublicUser, accountID string) (Account, error) {
	return defaultAccountService().UnbindProfile(actor, accountID)
}

func StartAccountCheck(actor identityservice.PublicUser, accountID string, input AccountCheckStartInput) (AccountCheckStart, error) {
	return defaultAccountService().StartLocalAccountCheck(actor, accountID, input)
}

func ApplyAccountCheckResult(actor identityservice.PublicUser, accountID string, input AccountCheckResultInput) (Account, error) {
	return defaultAccountService().ApplyLocalAccountCheckResult(actor, accountID, input)
}

func ListAccountPage(actor identityservice.PublicUser, filter AccountFilter, page, size int) (dto.AccountPage, error) {
	return defaultAccountService().ListAccountPage(actor, filter, page, size)
}
