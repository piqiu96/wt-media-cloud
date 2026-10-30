package repository

import (
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/dto"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"os"
	"strings"
	"testing"
	"time"
)

// Temporary tables shadow the development tables on a single connection. No
// existing account, relation, browser profile or credential is modified.
func TestMySQLCheckAndMetadataSettlement(t *testing.T) {
	dsn := os.Getenv("WT_MEDIA_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set WT_MEDIA_MYSQL_TEST_DSN for real MySQL validation")
	}
	db, err := gorm.Open(mysqlgorm.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	for _, table := range []string{"media_accounts", "media_account_categories", "media_account_tags"} {
		var name, ddl string
		if err := queryRow(db, "SHOW CREATE TABLE "+table).Scan(&name, &ddl); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(ddl, "\n")
		kept := []string{}
		for _, line := range lines {
			if !strings.Contains(line, "CONSTRAINT ") {
				kept = append(kept, line)
			}
		}
		ddl = strings.Join(kept, "\n")
		ddl = strings.Replace(ddl, ",\n)", "\n)", 1)
		ddl = strings.Replace(ddl, "CREATE TABLE", "CREATE TEMPORARY TABLE", 1)
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Truncate(time.Microsecond)
	record := model.AccountRecord{Account: model.Account{UserID: 3, Platform: model.PlatformBilibili, BrowserProfileID: "27", Enabled: true, IdentificationStatus: model.IdentificationPending, PlatformLoginStatus: model.PlatformLoginUnknown, Tags: []string{"初始"}, CreatedAt: now, UpdatedAt: now}, OriginalCookie: "test-original"}
	accountID, err := create(db, record)
	if err != nil {
		t.Fatal(err)
	}
	record.ID = accountID
	record.CheckOperationID = "check-first"
	record.CheckProfileID = "27"
	record.CheckStartedAt = &now
	if err = startCheck(db, record); err != nil {
		t.Fatal(err)
	}
	// Metadata changed after a check started must survive its later settlement.
	remark := "检查时编辑的备注"
	tags := []string{"矩阵"}
	record.Remark = remark
	record.Tags = tags
	if err = updateMetadata(db, record, dto.UpdateAccountInput{Remark: &remark, Tags: &tags}); err != nil {
		t.Fatal(err)
	}
	record.Remark = "stale-remark"
	record.OriginalCookie = "must-not-replace"
	record.PlatformAccountID = "real-readback-uid"
	record.PlatformName = "实时昵称"
	record.PlatformLoginStatus = model.PlatformLoginNormal
	record.LastCheckStatus = "completed"
	record.LastCheckAt = &now
	if err = completeCheck(db, record); err != nil {
		t.Fatal(err)
	}
	var gotRemark, gotCookie, gotUID string
	if err = queryRow(db, "SELECT remark,original_cookie,platform_account_id FROM media_accounts WHERE id=?", accountID).Scan(&gotRemark, &gotCookie, &gotUID); err != nil {
		t.Fatal(err)
	}
	if gotRemark != remark || gotCookie != "test-original" || gotUID != "real-readback-uid" {
		t.Fatalf("settlement replaced unrelated facts: remark=%q uid=%q", gotRemark, gotUID)
	}
	for _, scenario := range []struct {
		query AccountFilter
		total int
	}{{AccountFilter{UserID: 3, AllTags: []string{"矩阵"}, Limit: 1}, 1}, {AccountFilter{UserID: 3, ExcludeTags: []string{"矩阵"}, Limit: 1}, 0}, {AccountFilter{UserID: 3, AnyTags: []string{"不存在"}, Limit: 1}, 0}, {AccountFilter{UserID: 3, View: "unbound", Limit: 1}, 0}, {AccountFilter{UserID: 3, View: "normal", Limit: 1}, 1}} {
		rows, counts, err := listPage(db, scenario.query)
		if err != nil {
			t.Fatal(err)
		}
		if counts.Total != scenario.total || len(rows) != scenario.total {
			t.Fatalf("filtered page/count disagree: rows=%d total=%d want=%d", len(rows), counts.Total, scenario.total)
		}
	}
	// The DB predicate (not a service-only pre-read) rejects duplicate and late writes.
	if err = completeCheck(db, record); !errors.Is(err, model.ErrStaleCheck) {
		t.Fatalf("duplicate completion=%v", err)
	}
	record.CheckOperationID = "check-new"
	if err = startCheck(db, record); err != nil {
		t.Fatal(err)
	}
	record.CheckOperationID = "check-first"
	if err = completeCheck(db, record); !errors.Is(err, model.ErrStaleCheck) {
		t.Fatalf("old operation=%v", err)
	}
	record.CheckOperationID = "check-new"
	if err = db.Exec("UPDATE media_accounts SET browser_profile_id=40 WHERE id=?", accountID).Error; err != nil {
		t.Fatal(err)
	}
	if err = completeCheck(db, record); !errors.Is(err, model.ErrStaleCheck) {
		t.Fatalf("changed target=%v", err)
	}
	// A relation write failure must roll back the account remark too.
	if err = db.Exec("DROP TEMPORARY TABLE media_account_tags").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("CREATE TEMPORARY TABLE media_account_tags (media_account_id BIGINT UNSIGNED PRIMARY KEY, tag_name VARCHAR(1) NOT NULL, created_at DATETIME(6) NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	record.Remark = "must-roll-back"
	record.Tags = []string{"a", "b"}
	if err = updateMetadata(db, record, dto.UpdateAccountInput{Remark: &record.Remark, Tags: &record.Tags}); err == nil {
		t.Fatal("expected relation insert failure")
	}
	if err = queryRow(db, "SELECT remark FROM media_accounts WHERE id=?", accountID).Scan(&gotRemark); err != nil {
		t.Fatal(err)
	}
	if gotRemark != remark {
		t.Fatalf("transaction did not roll back remark: %q", gotRemark)
	}
}
