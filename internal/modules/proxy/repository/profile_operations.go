package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	"github.com/wt-media/wt-media-cloud/internal/shared/id"
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
	Operation    *ProfileProxyReceipt
}

type ProfileProxyReceipt struct {
	ID      string                 `json:"operation_id"`
	Action  string                 `json:"action"`
	ProxyID sharedidentity.ProxyID `json:"proxy_id"`
	Outcome string                 `json:"outcome"`
	// The non-secret connection sent to Bit at operation start. An acknowledged
	// write can project these fields without a second Bit read or a manual sync.
	ProxyProtocol string `json:"proxy_protocol,omitempty"`
	ProxyHost     string `json:"proxy_host,omitempty"`
	ProxyPort     int    `json:"proxy_port,omitempty"`
}

func reserveReceipt(tx *gorm.DB, target *ProfileProxyTarget, action, proxyID string, proxy *model.Proxy) error {
	return storeReceipt(tx, target, action, proxyID, "pending", proxy)
}

func storeReceipt(tx *gorm.DB, target *ProfileProxyTarget, action, proxyID, outcome string, proxy *model.Proxy) error {
	target.Operation = &ProfileProxyReceipt{ID: id.NewID("proxyop"), Action: action, ProxyID: sharedidentity.ProxyID(proxyID), Outcome: outcome}
	if action == "apply" && proxy != nil {
		target.Operation.ProxyProtocol = string(proxy.Protocol)
		target.Operation.ProxyHost = proxy.Host
		target.Operation.ProxyPort = proxy.Port
	}
	value, err := json.Marshal(target.Operation)
	if err != nil {
		return err
	}
	return tx.Exec(`UPDATE browser_profiles SET proxy_operation = ? WHERE id = ?`, string(value), target.ProfileID).Error
}

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
		if reservedProxyID.String != "" || reservedSince != nil {
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
		if err := tx.Raw(`SELECT COUNT(*) FROM browser_profiles WHERE local_state IN ('active', 'local_missing') AND id <> ? AND (proxy_id = ? OR reserved_proxy_id = ?)`, profileID, proxyID, proxyID).Scan(&occupied).Error; err != nil {
			return err
		}
		if occupied >= normalizedQuota(proxy.MaxProfiles) {
			return ErrQuotaExceeded
		}
		if err := tx.Exec(`UPDATE browser_profiles SET reserved_proxy_id = ?, reserved_since = ?, updated_at = ? WHERE id = ?`, proxyID, now, now, profileID).Error; err != nil {
			return err
		}
		return reserveReceipt(tx, &target, "apply", proxyID, &proxy)
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
		if reservedProxyID.String != "" || reservedSince != nil {
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
		return reserveReceipt(tx, &target, "unbind", proxyID.String, nil)
	})
	return target, proxy, err
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
		var proxyID, reservedProxyID sql.NullString
		var gotTeam *sharedidentity.TeamID
		if err := tx.Raw(`SELECT id, bit_profile_id, user_id, team_id, proxy_id, reserved_proxy_id FROM browser_profiles WHERE id = ? FOR UPDATE`, profileID).Row().
			Scan(&target.ProfileID, &target.BitProfileID, &target.UserID, &gotTeam, &proxyID, &reservedProxyID); err != nil {
			return err
		}
		if reservedProxyID.Valid {
			return ErrProxyOperationBusy
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
		if err := tx.Exec(`UPDATE browser_profiles SET proxy_id = NULL, reserved_proxy_id = NULL, reserved_since = NULL, updated_at = ? WHERE id = ?`, now, profileID).Error; err != nil {
			return err
		}
		return storeReceipt(tx, &target, "release", proxyID.String, "success", nil)
	})
	return target, proxy, err
}

// Read recovery facts only; the handler checks owner scope before returning credentials.
func GetProfileProxyOperation(profileID string) (ProfileProxyTarget, model.Proxy, error) {
	var target ProfileProxyTarget
	var raw []byte
	err := database.DB().Raw(`SELECT id, bit_profile_id, user_id, team_id, proxy_operation FROM browser_profiles WHERE id = ?`, profileID).Row().Scan(&target.ProfileID, &target.BitProfileID, &target.UserID, &target.TeamID, &raw)
	if err != nil {
		return target, model.Proxy{}, err
	}
	if len(raw) == 0 {
		return target, model.Proxy{}, ErrProxyOperationStale
	}
	if err := json.Unmarshal(raw, &target.Operation); err != nil {
		return target, model.Proxy{}, err
	}
	if target.Operation.Outcome != "pending" {
		return target, model.Proxy{ID: string(target.Operation.ProxyID)}, nil
	}
	proxy, found, err := FindByID(string(target.Operation.ProxyID))
	if err == nil && !found {
		err = gorm.ErrRecordNotFound
	}
	return target, proxy, err
}

// The operation ID fences late responses; the receipt makes settlement retry idempotent.
func ResolveProfileProxyOperationReceipt(profileID, operationID, action, proxyID, outcome string, now time.Time) error {
	return resolveProfileProxyOperationReceipt(database.DB(), profileID, operationID, action, proxyID, outcome, now)
}

func resolveProfileProxyOperationReceipt(db *gorm.DB, profileID, operationID, action, proxyID, outcome string, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var raw []byte
		var reserved sql.NullString
		if err := tx.Raw(`SELECT proxy_operation, reserved_proxy_id FROM browser_profiles WHERE id = ? FOR UPDATE`, profileID).Row().Scan(&raw, &reserved); err != nil {
			return err
		}
		var receipt ProfileProxyReceipt
		if len(raw) == 0 {
			return ErrProxyOperationStale
		}
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return err
		}
		if receipt.ID != operationID || operationID == "" || receipt.Action != action || string(receipt.ProxyID) != proxyID {
			return ErrProxyOperationStale
		}
		if receipt.Outcome != "pending" {
			if receipt.Outcome == outcome {
				return nil
			}
			return ErrProxyOperationStale
		}
		if reserved.String != proxyID {
			return ErrProxyOperationStale
		}
		if outcome == "success" {
			var formal any = proxyID
			if action == "unbind" {
				formal = nil
			}
			update := `UPDATE browser_profiles SET proxy_id = ? WHERE id = ?`
			args := []any{formal, profileID}
			if action == "apply" && receipt.ProxyProtocol != "" && receipt.ProxyHost != "" && receipt.ProxyPort > 0 {
				update = `UPDATE browser_profiles SET proxy_id = ?, bit_proxy_type = ?, bit_proxy_host = ?, bit_proxy_port = ? WHERE id = ?`
				args = []any{formal, receipt.ProxyProtocol, receipt.ProxyHost, receipt.ProxyPort, profileID}
			} else if action == "unbind" {
				update = `UPDATE browser_profiles SET proxy_id = NULL, bit_proxy_type = 'noproxy', bit_proxy_host = NULL, bit_proxy_port = 0 WHERE id = ?`
				args = []any{profileID}
			}
			if err := tx.Exec(update, args...).Error; err != nil {
				return err
			}
		}
		receipt.Outcome = outcome
		value, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		return tx.Exec(`UPDATE browser_profiles SET reserved_proxy_id = NULL, reserved_since = NULL, proxy_operation = ?, updated_at = ? WHERE id = ?`, string(value), now, profileID).Error
	})
}
