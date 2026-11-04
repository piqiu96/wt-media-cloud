package repository

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestMaterialPageFiltersKeepViewerSeparateFromOwner(t *testing.T) {
	team := identity.TeamID(7)
	lower := int64(10)
	upper := int64(100)
	from := time.Now()
	before := from.Add(24 * time.Hour)
	conditions, args := materialConditions(MaterialFilter{TeamID: &team, OwnerID: 9, UsageStatus: "active", UserID: 4, CategoryID: 3, MyUsage: "unused", TeamUsage: "used", PublishedFrom: &from, PublishedBefore: &before, LikeCountMin: &lower, LikeCountMax: &upper})
	where := strings.Join(conditions, " AND ")
	for _, required := range []string{"m.team_id = ?", "s.category_id = ?", "o.operator_id = ?", "o.team_id = m.team_id", "owner_usage.user_id=?", "owner_usage.status='active'", "s.published_at >= ?", "s.published_at < ?", "s.like_count >= ?", "s.like_count <= ?"} {
		if !strings.Contains(where, required) {
			t.Fatalf("missing %s in %s", required, where)
		}
	}
	if args[2] != identity.UserID(4) || args[3] != identity.UserID(9) {
		t.Fatalf("viewer/owner args=%v", args)
	}
}
func TestMaterialPageUsesDatabaseOffsetRatherThanCappedSlice(t *testing.T) {
	db, mock := newMockGORM(t)
	query := "SELECT " + materialProjectionColumns + " FROM materials m JOIN source_contents s ON s.id = m.source_content_id ORDER BY s.published_at DESC, m.id DESC LIMIT 20 OFFSET 260"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows(materialColumns()))
	rows, err := listMaterials(db, MaterialFilter{Page: 14, PageSize: 20})
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
