package repository

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	sharedidentity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

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

	var userID int64
	var teamID int64
	if err := database.DB().Raw(`SELECT id, team_id FROM users WHERE team_id IS NOT NULL ORDER BY id LIMIT 1`).Row().Scan(&userID, &teamID); err != nil {
		t.Skipf("no team-bound user available: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	proxyOne := "proxy_test_" + suffix
	proxyTwo := proxyOne + "_two"
	bitProfileOne := "profile_test_" + suffix
	bitProfileTwo := bitProfileOne + "_two"
	team := sharedidentity.TeamID(teamID)
	insertProxy := `INSERT INTO proxies (id, team_id, name, source, protocol, host, port, enabled, max_profiles, last_check_at, last_check_result, last_check_message, created_at, updated_at) VALUES (?, ?, ?, 'static', 'http', ?, 8080, 1, ?, ?, 'ok', '', ?, ?)`
	if err := database.DB().Exec(insertProxy, proxyOne, teamID, "并发测试一", "10.9.0.1", 1, now, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB().Exec(insertProxy, proxyTwo, teamID, "并发测试二", "10.9.0.2", 1, now, now, now).Error; err != nil {
		t.Fatal(err)
	}
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
	if _, _, err := ReserveProfileProxy(profileOne, proxyOne, team, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReserveProfileProxy(profileOne, proxyTwo, team, now); !errors.Is(err, ErrProxyOperationBusy) {
		t.Fatalf("same-profile second reservation error=%v, want busy", err)
	}
	if _, err := ResolveProfileProxyOperation(profileOne, proxyOne, true, false, now); err != nil {
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
		_ = database.DB().Raw(`SELECT COALESCE(reserved_proxy_id, '') FROM browser_profiles WHERE id = ?`, profileID).Row().Scan(&reserved)
		if reserved == proxyTwo {
			winnerProfile = profileID
		}
	}
	if winnerProfile == "" {
		t.Fatal("no reservation winner persisted")
	}
	if _, err := ResolveProfileProxyOperation(winnerProfile, proxyTwo, true, false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var reserved int
	if err := database.DB().Raw(`SELECT COUNT(*) FROM browser_profiles WHERE id IN (?, ?) AND reserved_since IS NOT NULL`, profileOne, profileTwo).Scan(&reserved).Error; err != nil || reserved != 0 {
		t.Fatalf("reservation cleanup count=%d err=%v", reserved, err)
	}
}
