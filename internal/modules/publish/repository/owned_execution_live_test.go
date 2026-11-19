package repository_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"strings"
	"sync"
	"testing"
	"time"
)

// Real owner services against an isolated SQL fixture. Full provider migrations
// and foreign keys are tested independently by TestLiveFullMigrationChain.
func ownedStore(t *testing.T) (repo.ExecutionStore, func(int) model.Task) {
	db := isolated(t)
	queries := []string{
		`CREATE TABLE users(id BIGINT UNSIGNED PRIMARY KEY,role VARCHAR(32),status VARCHAR(16),team_id BIGINT,bit_main_user_id VARCHAR(255))`,
		`INSERT INTO users VALUES(9,'operator','enabled',7,'main-9')`,
		`CREATE TABLE categories(id BIGINT PRIMARY KEY,enabled BOOL)`, `INSERT INTO categories VALUES(1,1)`,
		`CREATE TABLE user_category_scopes(user_id BIGINT,category_id BIGINT,PRIMARY KEY(user_id,category_id))`, `INSERT INTO user_category_scopes VALUES(9,1)`,
		`CREATE TABLE media_account_categories(media_account_id BIGINT,category_id BIGINT,PRIMARY KEY(media_account_id,category_id))`,
		`ALTER TABLE media_accounts ADD user_id BIGINT,ADD team_id BIGINT,ADD platform VARCHAR(32),ADD platform_account_id VARCHAR(255),ADD browser_profile_id BIGINT,ADD enabled BOOL,ADD identification_status VARCHAR(32),ADD platform_login_status VARCHAR(32)`,
		`ALTER TABLE browser_profiles ADD user_id BIGINT,ADD team_id BIGINT,ADD bit_profile_id VARCHAR(255),ADD bit_main_user_id VARCHAR(255),ADD bit_user_id VARCHAR(255),ADD bit_name VARCHAR(255),ADD bit_seq INT,ADD bit_group_id VARCHAR(255),ADD bit_group_name VARCHAR(255),ADD bit_profile_status VARCHAR(64),ADD bit_updated_at VARCHAR(64),ADD bit_proxy_type VARCHAR(32),ADD bit_proxy_host VARCHAR(255),ADD bit_proxy_port INT,ADD proxy_id BIGINT,ADD reserved_proxy_id BIGINT,ADD reserved_since DATETIME(6),ADD bit_remark VARCHAR(255),ADD remark VARCHAR(255),ADD enabled BOOL,ADD local_state VARCHAR(32),ADD last_synced_at DATETIME(6),ADD created_at DATETIME(6),ADD updated_at DATETIME(6),ADD cookie_operation_id VARCHAR(64)`,
		`ALTER TABLE composite_outputs ADD team_id BIGINT,ADD category_id BIGINT,ADD operator_id BIGINT,ADD material_id BIGINT,ADD material_usage_id BIGINT,ADD compose_task_id BIGINT,ADD compose_strategy_id BIGINT,ADD template_code VARCHAR(64),ADD template_version INT,ADD object_key VARCHAR(512),ADD file_size_bytes BIGINT,ADD sha256 CHAR(64),ADD media_json JSON,ADD publish_status VARCHAR(32),ADD created_by BIGINT,ADD created_at DATETIME(6),ADD updated_at DATETIME(6),ADD name VARCHAR(255),ADD file_name VARCHAR(255)`,
		`CREATE TABLE compose_tasks(id BIGINT PRIMARY KEY,platforms_json JSON)`,
	}
	for _, q := range queries {
		if e := db.Exec(q).Error; e != nil {
			t.Fatal(e)
		}
	}
	for n := 0; n < 5; n++ {
		profile, account, output := 37+n, 12+n, 42+n
		if n > 0 {
			if e := db.Exec("INSERT INTO media_accounts(id) VALUES(?)", account).Error; e != nil {
				t.Fatal(e)
			}
			if n > 2 {
				if e := db.Exec("INSERT INTO composite_outputs(id) VALUES(?)", output).Error; e != nil {
					t.Fatal(e)
				}
			}
		}
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO browser_profiles(id,user_id,team_id,bit_profile_id,bit_main_user_id,bit_user_id,bit_name,bit_seq,bit_proxy_port,enabled,local_state,last_synced_at,created_at,updated_at) VALUES(?,9,7,?,'main-9','bit-user','test',1,0,1,'active',NOW(6),NOW(6),NOW(6))`, []any{profile, fmt.Sprint("bit-", profile)}},
			{`UPDATE media_accounts SET user_id=9,team_id=7,platform='baidu',platform_account_id=?,browser_profile_id=?,enabled=1,identification_status='identified',platform_login_status='normal' WHERE id=?`, []any{fmt.Sprint("uid-", n), profile, account}},
			{`INSERT INTO media_account_categories VALUES(?,1)`, []any{account}},
			{`UPDATE composite_outputs SET team_id=7,category_id=1,operator_id=9,material_id=1,compose_task_id=?,compose_strategy_id=1,template_code='test',template_version=1,object_key=?,file_size_bytes=16,sha256=?,media_json='{}',publish_status='pending_publish',created_by=9,created_at=NOW(6),updated_at=NOW(6) WHERE id=?`, []any{output, fmt.Sprint("video-", n), strings.Repeat("a", 64), output}},
			{`INSERT INTO compose_tasks VALUES(?, '["baidu"]')`, []any{output}},
		} {
			if e := db.Exec(q.sql, q.args...).Error; e != nil {
				t.Fatal(e)
			}
		}
	}
	s := service.NewExecutionStore(db)
	makeTask := func(n int) model.Task {
		v := task(fmt.Sprint("owned-", n), int64(42+n), 1)
		v.MediaAccountID = uint64(12 + n)
		v.PlatformAccountUID = fmt.Sprint("uid-", n)
		v.ResourceSnapshot, _ = json.Marshal(map[string]any{"browser_profile_id": fmt.Sprint(37 + n), "video_sha256": strings.Repeat("a", 64), "video_size": 16, "cover_sha256": fmt.Sprintf("%064d", 1)})
		v, e := s.Create(v)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	return s, makeTask
}
func TestLiveOwnedInputAndUserCapAcrossDifferentProfiles(t *testing.T) {
	s, makeTask := ownedStore(t)
	now := time.Now().UTC()
	for n := 0; n < 5; n++ {
		v := makeTask(n)
		if _, e := s.Start(v, 1, 9, now); e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	results := make(chan *model.Claim, 10)
	errs := make(chan error, 10)
	for n := 0; n < 10; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			v, e := s.Claim(9, fmt.Sprint("concurrent-", n), now)
			results <- v
			errs <- e
		}(n)
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	count := 0
	for c := range results {
		if c != nil {
			count++
			if c.Attempt.Input.BitMainUserID != "main-9" || c.Attempt.Input.CoverSize != 3 {
				t.Fatalf("%+v", c)
			}
		}
	}
	if count != 3 {
		t.Fatalf("per-owner running count=%d", count)
	}
	if e := s.DB.Exec("UPDATE publication_attempt SET status='awaiting_human' WHERE id=(SELECT id FROM (SELECT MIN(id) AS id FROM publication_attempt) a)").Error; e != nil {
		t.Fatal(e)
	}
	if e := s.InterruptStale(now.Add(91 * time.Second)); e != nil {
		t.Fatal(e)
	}
	if c, e := s.Claim(9, "after-paused-scenes", now.Add(92*time.Second)); e != nil || c != nil {
		t.Fatalf("unresolved scenes bypassed user cap: %+v %v", c, e)
	}

}
func TestLiveOwnedCookieBusyAndBindingChangeDenyNewWork(t *testing.T) {
	s, makeTask := ownedStore(t)
	v := makeTask(0)
	now := time.Now().UTC()
	s.DB.Exec("UPDATE browser_profiles SET cookie_operation_id='active' WHERE id=37")
	if _, e := s.Start(v, 1, 9, now); !errors.Is(e, repo.ErrConflict) {
		t.Fatalf("cookie mutation overlapped: %v", e)
	}
	s.DB.Exec("UPDATE browser_profiles SET cookie_operation_id=NULL WHERE id=37")
	if _, e := s.Start(v, 1, 9, now); e != nil {
		t.Fatal(e)
	}
	c, e := s.Claim(9, "owned-claim", now)
	if e != nil || c == nil {
		t.Fatalf("%+v %v", c, e)
	}
	s.DB.Exec("UPDATE media_accounts SET browser_profile_id=38 WHERE id=12")
	if f, e := s.Report(c.Task, 9, model.ExecutionReport{AttemptID: c.Attempt.ID, Seq: 1, Stage: model.AttemptStagePreparing}, true, now); e != nil || !f.StopRequested || f.AllowSubmit || f.StopReason != model.ExecutionInputChanged {
		t.Fatalf("changed binding did not stop: %+v %v", f, e)
	}
	if e := repo.ProfileExecutionBusy(s.DB, "37"); !errors.Is(e, repo.ErrConflict) {
		t.Fatal(e)
	}
}
