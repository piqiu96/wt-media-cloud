package repository

import (
	"errors"
	"github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/model"
	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMySQLCookieTransactionAndSharedWindow(t *testing.T) {
	dsn := os.Getenv("WT_MEDIA_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set WT_MEDIA_MYSQL_TEST_DSN")
	}
	db, e := gorm.Open(mysqlgorm.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	for _, table := range []string{"media_accounts", "media_account_categories", "media_account_tags"} {
		var name, ddl string
		if e = queryRow(db, "SHOW CREATE TABLE "+table).Scan(&name, &ddl); e != nil {
			t.Fatal(e)
		}
		lines := strings.Split(ddl, "\n")
		kept := []string{}
		for _, line := range lines {
			if !strings.Contains(line, "CONSTRAINT ") {
				kept = append(kept, line)
			}
		}
		ddl = strings.Replace(strings.Replace(strings.Join(kept, "\n"), ",\n)", "\n)", 1), "CREATE TABLE", "CREATE TEMPORARY TABLE", 1)
		if e = db.Exec(ddl).Error; e != nil {
			t.Fatal(e)
		}
	}
	if e = db.Exec(`CREATE TEMPORARY TABLE browser_profiles (id BIGINT PRIMARY KEY,user_id BIGINT,bit_profile_id VARCHAR(64),enabled BOOLEAN,cookie_operation_id VARCHAR(64),reserved_proxy_id BIGINT NULL)`).Error; e != nil {
		t.Fatal(e)
	}
	if e = db.Exec(`INSERT INTO browser_profiles VALUES(1,3,'bit',1,NULL,NULL),(2,3,'bit2',1,NULL,NULL)`).Error; e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	record := model.AccountRecord{Account: model.Account{UserID: 3, Platform: model.PlatformBaidu, BrowserProfileID: "1", Enabled: true, IdentificationStatus: model.IdentificationPending, PlatformLoginStatus: model.PlatformLoginUnknown, Remark: "preserve metadata", CreatedAt: now, UpdatedAt: now}, ActiveCookie: "old"}
	id, e := create(db, record)
	if e != nil {
		t.Fatal(e)
	}
	_, e = mutateCookie(db, id, 3, func(r *model.AccountRecord, lock *string) error {
		r.CookieUpdate = &model.CookieUpdate{OperationID: "op", BrowserProfileID: "1", BitProfileID: "bit", Status: "running"}
		*lock = "op"
		r.BeforeCookie = `[{"name":"old","domain":".baidu.com","path":"/","httpOnly":true}]`
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	record.ID = id
	record.BrowserProfileID = "2"
	if e = setProfile(db, record); !errors.Is(e, model.ErrCookieConflict) {
		t.Fatal("binding bypassed lock", e)
	}
	_, e = mutateCookie(db, id, 3, func(r *model.AccountRecord, lock *string) error {
		r.ActiveCookie = "bad"
		*lock = ""
		return model.ErrCookieConflict
	})
	if !errors.Is(e, model.ErrCookieConflict) {
		t.Fatal(e)
	}
	got, found, e := findByID(db, id)
	if e != nil || !found || got.ActiveCookie != "old" || got.BeforeCookie == "" || got.Remark != "preserve metadata" {
		t.Fatal("rollback or preservation failed", e)
	}
	// Different platform/account still shares the same window operation mutex.
	record.BrowserProfileID = "1"
	record.Platform = model.PlatformBilibili
	second, e := create(db, record)
	if e != nil {
		t.Fatal(e)
	}
	_, e = mutateCookie(db, second, 3, func(r *model.AccountRecord, lock *string) error {
		if *lock != "" {
			return model.ErrCookieConflict
		}
		return nil
	})
	if !errors.Is(e, model.ErrCookieConflict) {
		t.Fatal("cross-account lock bypassed", e)
	}
	_, e = mutateCookie(db, id, 3, func(r *model.AccountRecord, lock *string) error {
		r.CookieUpdate.Status = "failed"
		*lock = ""
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	record.Platform = model.PlatformBaidu
	record.BrowserProfileID = "2"
	if e = setProfile(db, record); e != nil {
		t.Fatal("settled lock not released", e)
	}
}
