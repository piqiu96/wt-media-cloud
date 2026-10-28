package repository

import (
	"regexp"
	"testing"

	identity "github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestListSourceViewsProjectsNamesAndFiltersMaterial(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	created := time.Date(2026, 9, 21, 14, 30, 0, 0, time.UTC)
	columns := []string{
		"id", "team_id", "category_id", "platform", "platform_content_id", "title", "description", "cover_url", "source_url",
		"author_id", "author_sec_uid", "author_uid", "author_home_url", "author_name", "source_type", "strategy_id", "crawl_task_id",
		"like_count", "favorite_count", "view_count", "comment_count", "share_count",
		"published_at", "status", "ignored_reason", "audit_note", "failure_reason", "material_id", "created_by", "updated_by", "audited_by", "audited_at",
		"created_at", "updated_at", "strategy_name", "crawl_task_name", "created_by_name", "updated_by_name", "audited_by_name",
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM source_contents s") + ".*LEFT JOIN crawl_tasks t.*WHERE s.material_id = ?").
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			int64(1), int64(10), nil, "douyin", "aweme-1", "Delta safe route", nil, "https://cover", "https://source",
			"author-id", "author-sec", "author-id", "https://www.douyin.com/user/author-sec?showSubTab=video&showTab=post", "Author", "strategy", int64(3), int64(23),
			int64(12000), int64(700), int64(340000), int64(560), int64(88),
			created, "material_created", nil, nil, nil, int64(17), int64(2), nil, nil, nil, created, created,
			"Delta hotspot", "Delta hotspot_20260921143000", "admin", "admin", "admin",
		))

	materialID := int64(17)
	items, err := listSources(testGORM(db), Filter{MaterialID: &materialID})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	item := items[0]
	if item.StrategyName != "Delta hotspot" || item.CrawlTaskName != "Delta hotspot_20260921143000" {
		t.Fatalf("unexpected source view: %+v", item)
	}
	if item.MaterialID == nil || *item.MaterialID != 17 {
		t.Fatalf("unexpected material ID: %+v", item.MaterialID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindSourceViewProjectsHistoricalTaskName(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	created := time.Date(2026, 9, 21, 14, 30, 0, 0, time.UTC)
	columns := []string{
		"id", "team_id", "category_id", "platform", "platform_content_id", "title", "description", "cover_url", "source_url",
		"author_id", "author_sec_uid", "author_uid", "author_home_url", "author_name", "source_type", "strategy_id", "crawl_task_id",
		"like_count", "favorite_count", "view_count", "comment_count", "share_count",
		"published_at", "status", "ignored_reason", "audit_note", "failure_reason", "material_id", "created_by", "updated_by", "audited_by", "audited_at",
		"created_at", "updated_at", "strategy_name", "crawl_task_name", "created_by_name", "updated_by_name", "audited_by_name",
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM source_contents s") + ".*WHERE s.id = ?").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			int64(9), int64(10), nil, "douyin", "aweme-9", "Historical item", nil, nil, "https://source",
			"author-id", nil, nil, nil, "Author", "strategy", int64(3), int64(23),
			int64(1), int64(2), int64(3), int64(4), int64(5),
			created, "pending", nil, nil, nil, nil, int64(2), nil, nil, nil, created, created,
			"Current name", "Historical name_20260921143000", "admin", "admin", "admin",
		))

	item, found, err := findSource(testGORM(db), 9)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("source view was not found")
	}
	if item.StrategyName != "Current name" || item.CrawlTaskName != "Historical name_20260921143000" {
		t.Fatalf("unexpected source view: %+v", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceViewConditionsBuildsCategoryMultiSelect(t *testing.T) {
	conditions, args := sourceViewConditions(Filter{CategoryIDs: []identity.CategoryID{1, 3}})
	if len(conditions) != 1 || conditions[0] != "s.category_id IN (?,?)" {
		t.Fatalf("conditions = %#v, want category IN clause", conditions)
	}
	if len(args) != 2 || args[0] != identity.CategoryID(1) || args[1] != identity.CategoryID(3) {
		t.Fatalf("args = %#v, want hy and sjz", args)
	}
	empty, emptyArgs := sourceViewConditions(Filter{CategoryIDs: []identity.CategoryID{0, 0}})
	if len(empty) != 0 || len(emptyArgs) != 0 {
		t.Fatalf("blank category ids should not filter: conditions=%#v args=%#v", empty, emptyArgs)
	}
}
