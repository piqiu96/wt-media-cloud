package repository

import (
	"errors"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func importTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("WT_MEDIA_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set WT_MEDIA_MYSQL_TEST_DSN")
	}
	db, e := gorm.Open(mysqlgorm.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	return db
}
func tempImportTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	for _, table := range []string{"media_account_import_batches", "media_accounts", "media_account_categories", "media_account_tags"} {
		var name, ddl string
		if e := queryRow(db, "SHOW CREATE TABLE "+table).Scan(&name, &ddl); e != nil {
			t.Fatal(e)
		}
		lines := []string{}
		for _, l := range strings.Split(ddl, "\n") {
			if !strings.Contains(l, "CONSTRAINT ") {
				lines = append(lines, l)
			}
		}
		ddl = strings.Replace(strings.Replace(strings.Join(lines, "\n"), ",\n)", "\n)", 1), "CREATE TABLE", "CREATE TEMPORARY TABLE", 1)
		if e := db.Exec(ddl).Error; e != nil {
			t.Fatal(e)
		}
	}
	for _, q := range []string{`CREATE TEMPORARY TABLE users (id BIGINT PRIMARY KEY,bit_main_user_id VARCHAR(64),nickname VARCHAR(64),username VARCHAR(64))`, `INSERT INTO users VALUES (3,'main','','test')`, `CREATE TEMPORARY TABLE browser_profiles (id BIGINT PRIMARY KEY,user_id BIGINT,bit_profile_id VARCHAR(64),bit_name VARCHAR(64),bit_main_user_id VARCHAR(64),bit_remark VARCHAR(128),cookie_operation_id VARCHAR(64),reserved_proxy_id BIGINT NULL,enabled BOOLEAN,local_state VARCHAR(32))`, `INSERT INTO browser_profiles VALUES(1,3,'bit','测试窗口','main','WT_IMPORT:1:3:1',NULL,NULL,1,'active')`} {
		if e := db.Exec(q).Error; e != nil {
			t.Fatal(e)
		}
	}
}
func TestMySQLImportAtomicTransferAndSharedMutex(t *testing.T) {
	db := importTestDB(t)
	tempImportTables(t, db)
	now := time.Now()
	b, e := createImport(db, model.ImportBatch{UserID: 3, RequestKey: "test", Platform: model.PlatformBaidu, Content: model.ImportContent{Items: []model.ImportItem{{RowNumber: 3, InputCookie: "BDUSS=test", Status: "running", Step: "applying", Attempt: 1, OperationID: "import:1:3:1"}}}, CreatedAt: now, UpdatedAt: now})
	if e != nil {
		t.Fatal(e)
	}
	_, e = mutateImport(db, b.ID, 3, func(b *model.ImportBatch, r model.ImportResources) error {
		_, e := r.ReserveProfile("1", "import:1:3:1", b.Platform, "WT_IMPORT:1:3:1")
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	_, e = mutateImport(db, b.ID, 3, func(b *model.ImportBatch, r model.ImportResources) error {
		_, e := r.ReserveProfile("1", "another-batch", b.Platform, "")
		return e
	})
	if !errors.Is(e, model.ErrImportConflict) {
		t.Fatal("mutex bypass", e)
	}
	_, e = mutateImport(db, b.ID, 3, func(b *model.ImportBatch, r model.ImportResources) error {
		_, e := r.CreateAccount(model.AccountRecord{Account: model.Account{UserID: 3, Platform: model.PlatformBaidu, PlatformAccountID: "real", PlatformName: "first", BrowserProfileID: "1", IdentificationStatus: model.IdentificationIdentified, PlatformLoginStatus: model.PlatformLoginNormal, Enabled: true, CreatedAt: now, UpdatedAt: now}, OriginalCookie: "BDUSS=test", ActiveCookie: `[{"name":"BDUSS","value":"readback"}]`, BeforeCookie: "[]"})
		if e != nil {
			return e
		}
		b.Content.Items[0].InputCookie = ""
		return model.ErrImportConflict
	})
	if !errors.Is(e, model.ErrImportConflict) {
		t.Fatal(e)
	}
	var count int
	queryRow(db, `SELECT COUNT(*) FROM media_accounts`).Scan(&count)
	if count != 0 {
		t.Fatal("account did not roll back")
	}
	got, _ := getImport(db, b.ID, 3)
	if got.Content.Items[0].InputCookie == "" {
		t.Fatal("input lost")
	}
	done, e := mutateImport(db, b.ID, 3, func(b *model.ImportBatch, r model.ImportResources) error {
		id, e := r.CreateAccount(model.AccountRecord{Account: model.Account{UserID: 3, Platform: model.PlatformBaidu, PlatformAccountID: "real", PlatformName: "real", BrowserProfileID: "1", IdentificationStatus: model.IdentificationIdentified, PlatformLoginStatus: model.PlatformLoginNormal, Enabled: true, CreatedAt: now, UpdatedAt: now}, OriginalCookie: "BDUSS=test", ActiveCookie: `[{"name":"BDUSS","value":"readback"}]`, BeforeCookie: "[]", CookieUpdate: &model.CookieUpdate{Status: "succeeded"}})
		if e != nil {
			return e
		}
		b.Content.Items[0].AccountID = id
		b.Content.Items[0].Status = "succeeded"
		b.Content.Items[0].InputCookie = ""
		return r.ReleaseProfile("1", "import:1:3:1")
	})
	if e != nil {
		t.Fatal(e)
	}
	record, found, e := findByID(db, done.Content.Items[0].AccountID)
	if e != nil || !found || record.OriginalCookie != "BDUSS=test" || record.BeforeCookie != "[]" || record.LastCheckStatus != "completed" {
		t.Fatal("transfer", e)
	}
	_, e = mutateImport(db, b.ID, 3, func(b *model.ImportBatch, r model.ImportResources) error {
		_, e := r.ReserveProfile("1", "cross-platform", model.PlatformBilibili, "")
		return e
	})
	if !errors.Is(e, model.ErrProfilePlatformTaken) {
		t.Fatal("official binding was ignored", e)
	}

}
func TestMySQLImportRevisionAndCapacity(t *testing.T) {
	db := importTestDB(t)
	now := time.Now()
	key := fmt.Sprintf("chg110-test-%d", now.UnixNano())
	items := []model.ImportItem{}
	// Measured real Baidu sample is 21,337 bytes; 98 such inputs stay below 2MiB.
	for row := 1; row <= 98; row++ {
		items = append(items, model.ImportItem{RowNumber: row, Status: "pending", Step: "waiting", InputCookie: "BDUSS=" + strings.Repeat("x", 21331)})
	}
	b, e := createImport(db, model.ImportBatch{UserID: 3, RequestKey: key, Platform: model.PlatformBaidu, Content: model.ImportContent{Items: items}, CreatedAt: now, UpdatedAt: now})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM media_account_import_batches WHERE id=? AND user_id=3 AND request_key=?`, b.ID, key)
	})
	var wins atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 6; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := mutateImport(db, b.ID, 3, func(b *model.ImportBatch, r model.ImportResources) error {
				if b.Revision != 0 {
					return model.ErrImportConflict
				}
				b.Content.Items[97].Status = "skipped"
				wins.Add(1)
				return nil
			})
			if e != nil && !errors.Is(e, model.ErrImportConflict) {
				t.Errorf("mutation: %v", e)
			}
		}()
	}
	wg.Wait()
	got, e := getImport(db, b.ID, 3)
	if e != nil || wins.Load() != 1 || got.Revision != 1 || got.Content.Items[97].RowNumber != 98 || got.Content.Items[0].InputCookie != items[0].InputCookie {
		t.Fatal("CAS/capacity", wins.Load(), e)
	}
	t.Logf("98 x 21337-byte samples persisted; six concurrent revision-0 writers => %d winner, final revision %d", wins.Load(), got.Revision)
}
