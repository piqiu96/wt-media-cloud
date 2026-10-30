package repository

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

func TestLiveDeleteMissingProfileKeepsAccountsAndReleasesProxy(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_OPERATING_RESOURCES") != "1" {
		t.Skip("requires an isolated migrated Cloud database")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	var isolated bool
	for _, item := range cfg.Databases {
		if item.Name == "primary" && strings.HasPrefix(item.Database, "wt_media_chg099_") {
			isolated = true
		}
	}
	if !isolated {
		t.Fatal("refusing to delete profile outside a CHG099 isolated database")
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	db := database.DB()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	teamName, username, bitID := "cleanup-team-"+suffix, "cleanup-user-"+suffix, "cleanup-bit-"+suffix
	if err := db.Exec(`INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`, teamName, now, now).Error; err != nil {
		t.Fatal(err)
	}
	var teamID, userID, profileID, proxyID int64
	if err := db.Raw(`SELECT id FROM operation_teams WHERE name = ?`, teamName).Row().Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO users (username, password_hash, role, status, team_id, bit_main_user_id, created_at, updated_at) VALUES (?, 'unused-test-hash', 'operator', 'enabled', ?, 'cleanup-main', ?, ?)`, username, teamID, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT id FROM users WHERE username = ?`, username).Row().Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO proxies (team_id, name, source, protocol, host, port, enabled, max_profiles, created_at, updated_at) VALUES (?, ?, 'static', 'http', '10.9.0.1', 8080, 1, 1, ?, ?)`, teamID, "cleanup-proxy-"+suffix, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT id FROM proxies WHERE name = ?`, "cleanup-proxy-"+suffix).Row().Scan(&proxyID); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO browser_profiles (user_id, team_id, bit_profile_id, bit_main_user_id, bit_user_id, bit_name, bit_seq, proxy_id, enabled, local_state, last_synced_at, created_at, updated_at) VALUES (?, ?, ?, 'cleanup-main', 'bit-user', '已删除窗口', 1, ?, 1, 'local_missing', ?, ?, ?)`, userID, teamID, bitID, proxyID, now, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT id FROM browser_profiles WHERE bit_profile_id = ?`, bitID).Row().Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	accountName := "cleanup-account-" + suffix
	if err := db.Exec(`INSERT INTO media_accounts (user_id, team_id, platform, platform_account_id, browser_profile_id, enabled, platform_login_status, created_at, updated_at) VALUES (?, ?, 'bilibili', ?, ?, 1, 'normal', ?, ?)`, userID, teamID, accountName, profileID, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := deleteProfile(db, fmt.Sprint(profileID), sharedidentity.UserID(userID), now); err != nil {
		t.Fatal(err)
	}
	var profileCount, accountCount, detachedCount, auditCount, occupied int
	for _, query := range []struct {
		sql  string
		args []any
		out  *int
	}{
		{`SELECT COUNT(*) FROM browser_profiles WHERE id = ?`, []any{profileID}, &profileCount},
		{`SELECT COUNT(*) FROM media_accounts WHERE platform_account_id = ?`, []any{accountName}, &accountCount},
		{`SELECT COUNT(*) FROM media_accounts WHERE platform_account_id = ? AND browser_profile_id IS NULL AND platform_login_status = 'unknown'`, []any{accountName}, &detachedCount},
		{`SELECT COUNT(*) FROM audit_logs WHERE target_type = 'browser_profile' AND target_id = ? AND action = 'bitbrowser.profile_missing.delete'`, []any{fmt.Sprint(profileID)}, &auditCount},
		{`SELECT COUNT(*) FROM browser_profiles WHERE proxy_id = ?`, []any{proxyID}, &occupied},
	} {
		if err := db.Raw(query.sql, query.args...).Row().Scan(query.out); err != nil {
			t.Fatal(err)
		}
	}
	if profileCount != 0 || accountCount != 1 || detachedCount != 1 || auditCount != 1 || occupied != 0 {
		t.Fatalf("profile=%d account=%d detached=%d audit=%d occupied=%d", profileCount, accountCount, detachedCount, auditCount, occupied)
	}
}
