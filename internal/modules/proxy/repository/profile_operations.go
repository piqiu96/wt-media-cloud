package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

var (
	ErrProxyOperationBusy  = errors.New("profile proxy operation is already in progress")
	ErrProxyOperationStale = errors.New("profile proxy operation reservation does not exist")
	ErrProxyNotAssignable  = errors.New("proxy is not assignable")
	ErrQuotaExceeded       = errors.New("proxy quota is exceeded")
)

type ProfileProxyTarget struct {
	ProfileID    string
	BitProfileID string
	UserID       sharedidentity.UserID
	TeamID       *sharedidentity.TeamID
}

const proxyReservationTTL = 10 * time.Minute

func ReserveProfileProxy(profileID, proxyID string, teamID sharedidentity.TeamID, now time.Time) (ProfileProxyTarget, model.Proxy, error) {
	var target ProfileProxyTarget
	var proxy model.Proxy
	err := database.DB().Transaction(func(tx *gorm.DB) error {
		row := tx.Raw(`SELECT id, bit_profile_id, user_id, team_id, proxy_id, reserved_proxy_id, reserved_since FROM browser_profiles WHERE id = ? AND local_state = 'active' AND enabled = 1 FOR UPDATE`, profileID).Row()
		var gotTeam *sharedidentity.TeamID
		var currentProxyID, reservedProxyID sql.NullString
		var reservedSince *time.Time
		if err := row.Scan(&target.ProfileID, &target.BitProfileID, &target.UserID, &gotTeam, &currentProxyID, &reservedProxyID, &reservedSince); err != nil {
			return err
		}
		target.TeamID = gotTeam
		if gotTeam == nil || *gotTeam != teamID {
			return gorm.ErrRecordNotFound
		}
		if reservedProxyID.String != "" && reservedSince != nil && now.Sub(*reservedSince) < proxyReservationTTL {
			return ErrProxyOperationBusy
		}
		var scanErr error
		proxy, scanErr = scanProxy(tx.Raw(`SELECT `+proxyColumns+` FROM proxies WHERE id = ? AND team_id = ? FOR UPDATE`, proxyID, teamID).Row())
		if scanErr != nil {
			return scanErr
		}
		if !proxy.Enabled || proxy.Expired(now) || proxy.LastCheckResult != model.CheckOK {
			return ErrProxyNotAssignable
		}
		var occupied int
		if err := tx.Raw(`SELECT COUNT(*) FROM browser_profiles WHERE local_state = 'active' AND id <> ? AND (proxy_id = ? OR reserved_proxy_id = ?)`, profileID, proxyID, proxyID).Scan(&occupied).Error; err != nil {
			return err
		}
		if occupied >= normalizedQuota(proxy.MaxProfiles) {
			return ErrQuotaExceeded
		}
		if err := tx.Exec(`UPDATE browser_profiles SET reserved_proxy_id = ?, reserved_since = ?, updated_at = ? WHERE id = ?`, proxyID, now, now, profileID).Error; err != nil {
			return err
		}
		return nil
	})
	return target, proxy, err
}

func ReserveProfileUnbind(profileID string, teamID sharedidentity.TeamID, now time.Time) (ProfileProxyTarget, model.Proxy, error) {
	var target ProfileProxyTarget
	var proxy model.Proxy
	err := database.DB().Transaction(func(tx *gorm.DB) error {
		row := tx.Raw(`SELECT id, bit_profile_id, user_id, team_id, proxy_id, reserved_proxy_id, reserved_since FROM browser_profiles WHERE id = ? AND local_state = 'active' FOR UPDATE`, profileID).Row()
		var gotTeam *sharedidentity.TeamID
		var proxyID, reservedProxyID sql.NullString
		var reservedSince *time.Time
		if err := row.Scan(&target.ProfileID, &target.BitProfileID, &target.UserID, &gotTeam, &proxyID, &reservedProxyID, &reservedSince); err != nil {
			return err
		}
		target.TeamID = gotTeam
		if gotTeam == nil || *gotTeam != teamID || proxyID.String == "" {
			return gorm.ErrRecordNotFound
		}
		if reservedProxyID.String != "" && reservedSince != nil && now.Sub(*reservedSince) < proxyReservationTTL {
			return ErrProxyOperationBusy
		}
		var scanErr error
		proxy, scanErr = scanProxy(tx.Raw(`SELECT `+proxyColumns+` FROM proxies WHERE id = ? FOR UPDATE`, proxyID.String).Row())
		if scanErr != nil {
			return scanErr
		}
		if err := tx.Exec(`UPDATE browser_profiles SET reserved_proxy_id = ?, reserved_since = ?, updated_at = ? WHERE id = ?`, proxyID.String, now, now, profileID).Error; err != nil {
			return err
		}
		return nil
	})
	return target, proxy, err
}

func ResolveProfileProxyOperation(profileID, proxyID string, apply, success bool, now time.Time) (ProfileProxyTarget, error) {
	var target ProfileProxyTarget
	err := database.DB().Transaction(func(tx *gorm.DB) error {
		row := tx.Raw(`SELECT id, bit_profile_id, user_id, team_id, reserved_proxy_id, reserved_since FROM browser_profiles WHERE id = ? FOR UPDATE`, profileID).Row()
		var gotTeam *sharedidentity.TeamID
		var reservedProxyID sql.NullString
		var reservedSince *time.Time
		if err := row.Scan(&target.ProfileID, &target.BitProfileID, &target.UserID, &gotTeam, &reservedProxyID, &reservedSince); err != nil {
			return err
		}
		target.TeamID = gotTeam
		if reservedProxyID.String != proxyID || reservedSince == nil {
			return ErrProxyOperationStale
		}
		if success && apply {
			if err := tx.Exec(`UPDATE browser_profiles SET proxy_id = ?, reserved_proxy_id = NULL, reserved_since = NULL, updated_at = ? WHERE id = ?`, proxyID, now, profileID).Error; err != nil {
				return err
			}
			return nil
		}
		if success && !apply {
			if err := tx.Exec(`UPDATE browser_profiles SET proxy_id = NULL, reserved_proxy_id = NULL, reserved_since = NULL, updated_at = ? WHERE id = ? AND proxy_id = ?`, now, profileID, proxyID).Error; err != nil {
				return err
			}
			return nil
		}
		return tx.Exec(`UPDATE browser_profiles SET reserved_proxy_id = NULL, reserved_since = NULL, updated_at = ? WHERE id = ?`, now, profileID).Error
	})
	return target, err
}

func CountReservedProfilesByProxyID(proxyID string) (int, error) {
	var count int
	err := database.DB().Raw(`SELECT COUNT(*) FROM browser_profiles WHERE reserved_proxy_id = ? AND local_state = 'active'`, proxyID).Scan(&count).Error
	return count, err
}

func normalizedQuota(value int) int {
	if value <= 0 {
		return 3
	}
	return value
}

func ReleaseProfileProxy(profileID string, teamID sharedidentity.TeamID, now time.Time) (ProfileProxyTarget, model.Proxy, error) {
	var target ProfileProxyTarget
	var proxy model.Proxy
	err := database.DB().Transaction(func(tx *gorm.DB) error {
		var proxyID sql.NullString
		var gotTeam *sharedidentity.TeamID
		if err := tx.Raw(`SELECT id, bit_profile_id, user_id, team_id, proxy_id FROM browser_profiles WHERE id = ? FOR UPDATE`, profileID).Row().
			Scan(&target.ProfileID, &target.BitProfileID, &target.UserID, &gotTeam, &proxyID); err != nil {
			return err
		}
		target.TeamID = gotTeam
		if gotTeam == nil || *gotTeam != teamID || proxyID.String == "" {
			return gorm.ErrRecordNotFound
		}
		var scanErr error
		proxy, scanErr = scanProxy(tx.Raw(`SELECT `+proxyColumns+` FROM proxies WHERE id = ?`, proxyID.String).Row())
		if scanErr != nil {
			return scanErr
		}
		return tx.Exec(`UPDATE browser_profiles SET proxy_id = NULL, reserved_proxy_id = NULL, reserved_since = NULL, updated_at = ? WHERE id = ?`, now, profileID).Error
	})
	return target, proxy, err
}
