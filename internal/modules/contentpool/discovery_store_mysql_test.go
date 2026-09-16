package contentpool

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestClaimPendingCrawlTaskLocksAndTransitionsExactlyOneTask(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC)
	created := now.Add(-time.Minute)
	columns := []string{"id", "team_id", "strategy_id", "schedule_key", "task_id", "task_type", "platform", "status", "snapshot_json", "stats_json", "result_json", "error_message", "started_at", "finished_at", "created_by", "created_at", "updated_at"}
	mock.ExpectBegin()
	mock.ExpectQuery("FROM crawl_tasks WHERE status = 'pending'.*FOR UPDATE SKIP LOCKED").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(9, 7, 3, "daily:2026-09-16:09:00", nil, "discovery_task", "douyin", "pending", []byte(`{"operation":"keyword","keyword":"demo"}`), []byte(`{}`), nil, nil, nil, nil, 2, created, created))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE crawl_tasks SET task_id=?,status='running',started_at=?,updated_at=? WHERE id=? AND status='pending'")).
		WithArgs("crawl-9", now, now, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	task, found, err := NewMySQLDiscoveryStore(db).ClaimPendingCrawlTask(now)
	if err != nil {
		t.Fatal(err)
	}
	if !found || task.ID != 9 || task.Status != CrawlRunning || task.TaskID != "crawl-9" || task.StartedAt == nil || !task.StartedAt.Equal(now) {
		t.Fatalf("claimed task=%+v found=%v", task, found)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimPendingCrawlTaskReturnsEmptyWithoutUpdate(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("FROM crawl_tasks WHERE status = 'pending'.*FOR UPDATE SKIP LOCKED").
		WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "strategy_id", "schedule_key", "task_id", "task_type", "platform", "status", "snapshot_json", "stats_json", "result_json", "error_message", "started_at", "finished_at", "created_by", "created_at", "updated_at"}))
	mock.ExpectRollback()

	_, found, err := NewMySQLDiscoveryStore(db).ClaimPendingCrawlTask(time.Now())
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
