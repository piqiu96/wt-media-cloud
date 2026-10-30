package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	profilebindingmodel "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/model"
	profilebindingrepository "github.com/wt-media/wt-media-cloud/internal/modules/profilebinding/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/proxy/model"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"gorm.io/gorm"
)

func insertLiveProxy(t *testing.T, query string, args ...any) string {
	t.Helper()
	var proxyID string
	err := database.DB().Connection(func(conn *gorm.DB) error {
		if err := conn.Exec(query, args...).Error; err != nil {
			return err
		}
		return conn.Raw(`SELECT LAST_INSERT_ID()`).Row().Scan(&proxyID)
	})
	if err != nil {
		t.Fatal(err)
	}
	return proxyID
}

func TestLiveProxyCheckResultRejectsChangedCredentials(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_OPERATING_RESOURCES") != "1" {
		t.Skip("set WT_MEDIA_TEST_OPERATING_RESOURCES=1 against an isolated migrated Cloud database")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	teamName := "CHG099 check " + suffix
	var proxyID string
	if err := database.DB().Exec(`INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`, teamName, now, now).Error; err != nil {
		t.Fatal(err)
	}
	var teamID int64
	if err := database.DB().Raw(`SELECT id FROM operation_teams WHERE name = ?`, teamName).Row().Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB().Exec(`DELETE FROM proxies WHERE id = ?`, proxyID)
		database.DB().Exec(`DELETE FROM operation_teams WHERE id = ?`, teamID)
	})
	proxyID = insertLiveProxy(t, `INSERT INTO proxies (team_id, name, source, protocol, host, port, username, password, enabled, max_profiles, created_at, updated_at) VALUES (?, 'check-test', 'static', 'socks5', '10.9.0.1', 1080, 'user', 'NewSecret', 1, 3, ?, ?)`, teamID, now, now)
	if applied, err := recordCheckResult(database.DB(), proxyID, "socks5", "10.9.0.1", 1080, "user", "OldSecret", "ok", "", now); err != nil || applied {
		t.Fatalf("old credential check applied=%v err=%v", applied, err)
	}
	var result *string
	if err := database.DB().Raw(`SELECT last_check_result FROM proxies WHERE id = ?`, proxyID).Row().Scan(&result); err != nil || (result != nil && *result == "ok") {
		t.Fatalf("late result survived credential change: result=%v err=%v", result, err)
	}
	if applied, err := recordCheckResult(database.DB(), proxyID, "socks5", "10.9.0.1", 1080, "user", "NewSecret", "ok", "", now); err != nil || !applied {
		t.Fatalf("current credential check applied=%v err=%v", applied, err)
	}
}

func TestLiveDynamicExtractionUpdatesOnlyCurrentConnection(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_OPERATING_RESOURCES") != "1" {
		t.Skip("set WT_MEDIA_TEST_OPERATING_RESOURCES=1 against an isolated migrated Cloud database")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	teamName := "CHG099 refresh " + suffix
	var proxyID string
	if err := database.DB().Exec(`INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`, teamName, now, now).Error; err != nil {
		t.Fatal(err)
	}
	var teamID int64
	if err := database.DB().Raw(`SELECT id FROM operation_teams WHERE name = ?`, teamName).Row().Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB().Exec(`DELETE FROM proxies WHERE id = ?`, proxyID)
		database.DB().Exec(`DELETE FROM operation_teams WHERE id = ?`, teamID)
	})
	proxyID = insertLiveProxy(t, `INSERT INTO proxies (team_id, name, source, protocol, host, port, username, password, extract_url, enabled, max_profiles, remark, created_at, updated_at) VALUES (?, 'latest name', 'api', 'http', '10.9.0.1', 8080, 'old', 'secret', 'https://supplier.example/new', 1, 9, 'new note', ?, ?)`, teamID, now, now)
	current, found, err := FindByID(proxyID)
	if err != nil || !found {
		t.Fatalf("read resource version found=%v err=%v", found, err)
	}
	encodedVersion, err := json.Marshal(current.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrippedVersion time.Time
	if err := json.Unmarshal(encodedVersion, &roundTrippedVersion); err != nil {
		t.Fatal(err)
	}
	connection := model.Proxy{Protocol: "socks5", Host: "10.9.0.2", Port: 1080}
	if applied, err := updateExtractedConnection(database.DB(), proxyID, "https://supplier.example/old", roundTrippedVersion, connection, now.Add(time.Second)); err != nil || applied {
		t.Fatalf("stale supplier result applied=%v err=%v", applied, err)
	}
	if applied, err := updateExtractedConnection(database.DB(), proxyID, "https://supplier.example/new", roundTrippedVersion, connection, now.Add(time.Second)); err != nil || !applied {
		t.Fatalf("current supplier result applied=%v err=%v", applied, err)
	}
	if applied, err := updateExtractedConnection(database.DB(), proxyID, "https://supplier.example/new", roundTrippedVersion, model.Proxy{Protocol: "http", Host: "10.9.0.3", Port: 8080}, now.Add(2*time.Second)); err != nil || applied {
		t.Fatalf("concurrent stale extraction applied=%v err=%v", applied, err)
	}
	var name, remark, host string
	var enabled bool
	var maxProfiles int
	var username, password *string
	if err := database.DB().Raw(`SELECT name, remark, enabled, max_profiles, host, username, password FROM proxies WHERE id = ?`, proxyID).Row().Scan(&name, &remark, &enabled, &maxProfiles, &host, &username, &password); err != nil {
		t.Fatal(err)
	}
	if name != "latest name" || remark != "new note" || !enabled || maxProfiles != 9 || host != "10.9.0.2" || username != nil || password != nil {
		t.Fatalf("extraction overwrote resource fields or retained credentials: name=%q remark=%q enabled=%v quota=%d host=%q username=%v password=%v", name, remark, enabled, maxProfiles, host, username, password)
	}
}

func TestLiveProfileProxyReservationConcurrencyAndFailureRelease(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_OPERATING_RESOURCES") != "1" {
		t.Skip("set WT_MEDIA_TEST_OPERATING_RESOURCES=1 against an isolated migrated Cloud database")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	teamName := "CHG099 live " + suffix
	username := "chg099_live_" + suffix
	if err := database.DB().Exec(`INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`, teamName, now, now).Error; err != nil {
		t.Fatal(err)
	}
	var teamID, userID int64
	if err := database.DB().Raw(`SELECT id FROM operation_teams WHERE name = ?`, teamName).Row().Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(`INSERT INTO users (username, password_hash, role, status, team_id, bit_main_user_id, created_at, updated_at) VALUES (?, 'unused-test-hash', 'operator', 'enabled', ?, 'main-test', ?, ?)`, username, teamID, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Raw(`SELECT id FROM users WHERE username = ?`, username).Row().Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB().Exec(`DELETE FROM users WHERE id = ?`, userID)
		database.DB().Exec(`DELETE FROM operation_teams WHERE id = ?`, teamID)
	})
	var proxyOne, proxyTwo string
	bitProfileOne := "profile_test_" + suffix
	bitProfileTwo := bitProfileOne + "_two"
	team := sharedidentity.TeamID(teamID)
	insertProxy := `INSERT INTO proxies (team_id, name, source, protocol, host, port, enabled, max_profiles, last_check_at, last_check_result, last_check_message, created_at, updated_at) VALUES (?, ?, 'static', 'http', ?, 8080, 1, ?, ?, 'ok', '', ?, ?)`
	proxyOne = insertLiveProxy(t, insertProxy, teamID, "并发测试一", "10.9.0.1", 1, now, now, now)
	proxyTwo = insertLiveProxy(t, insertProxy, teamID, "并发测试二", "10.9.0.2", 1, now, now, now)
	insertProfile := `INSERT INTO browser_profiles (user_id, team_id, bit_profile_id, bit_main_user_id, bit_user_id, bit_name, bit_seq, enabled, local_state, last_synced_at, created_at, updated_at) VALUES (?, ?, ?, 'main-test', ?, ?, 1, 1, 'active', ?, ?, ?)`
	if err := database.DB().Exec(insertProfile, userID, teamID, "bit-"+bitProfileOne, "bit-user-1", "并发窗口一", now, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(insertProfile, userID, teamID, "bit-"+bitProfileTwo, "bit-user-2", "并发窗口二", now, now, now).Error; err != nil {
		t.Fatal(err)
	}
	var profileOneID, profileTwoID int64
	if err := database.DB().Raw(`SELECT id FROM browser_profiles WHERE bit_profile_id = ?`, "bit-"+bitProfileOne).Scan(&profileOneID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Raw(`SELECT id FROM browser_profiles WHERE bit_profile_id = ?`, "bit-"+bitProfileTwo).Scan(&profileTwoID).Error; err != nil {
		t.Fatal(err)
	}
	profileOne := fmt.Sprint(profileOneID)
	profileTwo := fmt.Sprint(profileTwoID)
	t.Cleanup(func() {
		database.DB().Exec(`DELETE FROM browser_profiles WHERE id IN (?, ?)`, profileOne, profileTwo)
		database.DB().Exec(`DELETE FROM proxies WHERE id IN (?, ?)`, proxyOne, proxyTwo)
	})

	// A second operation for the same window must fail while the first reservation exists.
	first, _, err := ReserveProfileProxy(profileOne, proxyOne, team, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Operation == nil || first.Operation.ID == "" {
		t.Fatal("reservation did not persist an operation receipt")
	}
	scan := profilebindingmodel.ProfileScan{ID: "scan-" + suffix, UserID: sharedidentity.UserID(userID), BitMainUserID: "main-test", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := profilebindingrepository.CreateScan(scan); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.DB().Exec(`DELETE FROM browser_profile_syncs WHERE id = ?`, scan.ID) })
	if err := profilebindingrepository.ApplyScan(scan, []string{"bit-" + bitProfileOne}, now); !errors.Is(err, profilebindingmodel.ErrProfileProxyPending) {
		t.Fatalf("sync overwrote a pending proxy operation: %v", err)
	}
	if _, _, err := ReserveProfileProxy(profileOne, proxyTwo, team, now.Add(11*time.Minute)); !errors.Is(err, ErrProxyOperationBusy) {
		t.Fatalf("reservation older than ten minutes was replaced: %v", err)
	}
	if err := ResolveProfileProxyOperationReceipt(profileOne, first.Operation.ID, "apply", proxyOne, "failed", now.Add(11*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ResolveProfileProxyOperationReceipt(profileOne, first.Operation.ID, "apply", proxyOne, "failed", now.Add(11*time.Minute)); err != nil {
		t.Fatalf("duplicate settlement should return the saved receipt: %v", err)
	}
	if _, _, err := ReserveProfileProxy(profileOne, proxyTwo, team, now.Add(12*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ResolveProfileProxyOperationReceipt(profileOne, first.Operation.ID, "apply", proxyOne, "success", now.Add(12*time.Minute)); !errors.Is(err, ErrProxyOperationStale) {
		t.Fatalf("old response settled a new reservation: %v", err)
	}
	if _, _, err := ReserveProfileProxy(profileOne, proxyOne, team, now); !errors.Is(err, ErrProxyOperationBusy) {
		t.Fatalf("same-profile second reservation error=%v, want busy", err)
	}
	current, _, err := GetProfileProxyOperation(profileOne)
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveProfileProxyOperationReceipt(profileOne, current.Operation.ID, "apply", proxyTwo, "failed", now.Add(12*time.Minute)); err != nil {
		t.Fatal(err)
	}

	// Two windows racing for the last slot: exactly one reservation can win.
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, item := range []struct {
		profile string
		proxy   string
	}{{profileOne, proxyTwo}, {profileTwo, proxyTwo}} {
		wait.Add(1)
		go func(profileID, proxyID string) {
			defer wait.Done()
			_, _, err := ReserveProfileProxy(profileID, proxyID, team, time.Now().UTC())
			results <- err
		}(item.profile, item.proxy)
	}
	wait.Wait()
	close(results)
	success, busy := 0, 0
	var winnerProfile string
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrQuotaExceeded):
			busy++
		default:
			t.Fatalf("unexpected reservation error: %v", err)
		}
	}
	if success != 1 || busy != 1 {
		t.Fatalf("reservation successes=%d quota_failures=%d, want 1/1", success, busy)
	}
	for _, profileID := range []string{profileOne, profileTwo} {
		var reserved string
		_ = database.DB().Raw(`SELECT COALESCE(CAST(reserved_proxy_id AS CHAR), '') FROM browser_profiles WHERE id = ?`, profileID).Row().Scan(&reserved)
		if reserved == proxyTwo {
			winnerProfile = profileID
		}
	}
	if winnerProfile == "" {
		t.Fatal("no reservation winner persisted")
	}
	winner, _, err := GetProfileProxyOperation(winnerProfile)
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveProfileProxyOperationReceipt(winnerProfile, winner.Operation.ID, "apply", proxyTwo, "failed", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var reserved int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM browser_profiles WHERE id IN (?, ?) AND reserved_since IS NOT NULL`, profileOne, profileTwo).Scan(&reserved).Error; err != nil || reserved != 0 {
		t.Fatalf("reservation cleanup count=%d err=%v", reserved, err)
	}
	assigned, _, err := ReserveProfileProxy(profileOne, proxyOne, team, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveProfileProxyOperationReceipt(profileOne, assigned.Operation.ID, "apply", proxyOne, "success", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var formalProxyID, observedType, observedHost string
	var observedPort int
	if err := database.DB().Raw(`SELECT CAST(proxy_id AS CHAR), COALESCE(bit_proxy_type, ''), COALESCE(bit_proxy_host, ''), bit_proxy_port FROM browser_profiles WHERE id = ?`, profileOne).
		Row().Scan(&formalProxyID, &observedType, &observedHost, &observedPort); err != nil {
		t.Fatal(err)
	}
	if formalProxyID != proxyOne || observedType != "http" || observedHost != "10.9.0.1" || observedPort != 8080 {
		t.Fatalf("acknowledged proxy was not projected: id=%s protocol=%s host=%s port=%d", formalProxyID, observedType, observedHost, observedPort)
	}
	if err := database.DB().Exec(`UPDATE browser_profiles SET local_state = 'local_missing' WHERE id = ?`, profileOne).Error; err != nil {
		t.Fatal(err)
	}
	if occupied, err := profilebindingrepository.CountProfilesByProxyID(proxyOne); err != nil || occupied != 1 {
		t.Fatalf("missing window lost its formal proxy slot: occupied=%d err=%v", occupied, err)
	}
	if _, _, err := ReserveProfileProxy(profileTwo, proxyOne, team, time.Now().UTC()); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("missing window slot could be overbooked: %v", err)
	}
	if err := database.DB().Exec(`UPDATE browser_profiles SET local_state = 'active' WHERE id = ?`, profileOne).Error; err != nil {
		t.Fatal(err)
	}
	released, _, err := ReleaseProfileProxy(profileOne, team, time.Now().UTC())
	if err != nil || released.Operation == nil || released.Operation.Action != "release" || released.Operation.Outcome != "success" {
		t.Fatalf("Cloud-only release did not record a terminal receipt: %+v %v", released.Operation, err)
	}
	recovered, _, err := GetProfileProxyOperation(profileOne)
	if err != nil || recovered.Operation == nil || recovered.Operation.ID != released.Operation.ID {
		t.Fatalf("release receipt cannot be recovered: %+v %v", recovered.Operation, err)
	}
}

func TestLiveProxyListPaginationWithSameCreationTime(t *testing.T) {
	if os.Getenv("WT_MEDIA_TEST_OPERATING_RESOURCES") != "1" {
		t.Skip("set WT_MEDIA_TEST_OPERATING_RESOURCES=1 against an isolated migrated Cloud database")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	teamName := "CHG099 paging " + suffix
	if err := database.DB().Exec(`INSERT INTO operation_teams (name, created_at, updated_at) VALUES (?, ?, ?)`, teamName, now, now).Error; err != nil {
		t.Fatal(err)
	}
	var teamID int64
	if err := database.DB().Raw(`SELECT id FROM operation_teams WHERE name = ?`, teamName).Row().Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB().Exec(`DELETE FROM proxies WHERE team_id = ?`, teamID)
		database.DB().Exec(`DELETE FROM operation_teams WHERE id = ?`, teamID)
	})
	for index := 0; index < 201; index++ {
		name := fmt.Sprintf("paging-%s-%03d", suffix, index)
		insertLiveProxy(t, `INSERT INTO proxies (team_id, name, source, protocol, host, port, enabled, max_profiles, created_at, updated_at) VALUES (?, ?, 'static', 'http', '127.0.0.1', 8080, 1, 1, ?, ?)`, teamID, name, now, now)
	}
	team := sharedidentity.TeamID(teamID)
	first, err := List(ProxyFilter{TeamID: &team, Limit: 200, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	second, err := List(ProxyFilter{TeamID: &team, Limit: 200, Offset: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 200 || len(second) != 1 || first[199].ID == second[0].ID {
		t.Fatalf("paging sizes/overlap: first=%d second=%d", len(first), len(second))
	}
	firstID, _ := strconv.ParseUint(first[0].ID, 10, 64)
	lastID, _ := strconv.ParseUint(first[199].ID, 10, 64)
	secondID, _ := strconv.ParseUint(second[0].ID, 10, 64)
	if firstID <= lastID || lastID <= secondID {
		t.Fatal("rows with identical creation time do not have a stable ID tie-breaker")
	}
}
