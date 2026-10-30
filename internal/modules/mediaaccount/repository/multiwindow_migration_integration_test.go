package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Only uniquely named fixture tables are changed in the configured development schema.
func TestMySQLMultiwindowMigrationAndLegacyImportRecovery(t *testing.T) {
	dsn := os.Getenv("WT_MEDIA_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set WT_MEDIA_MYSQL_TEST_DSN")
	}
	db, e := gorm.Open(mysqlgorm.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	suffix := fmt.Sprint(time.Now().UnixNano())
	accounts := "chg114_accounts_" + suffix
	batches := "chg114_batches_" + suffix
	fk := "chg114_fk_" + suffix
	check := "chg114_check_" + suffix
	exec := func(q string, args ...any) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE ` + accounts + ` (id BIGINT PRIMARY KEY,user_id BIGINT NOT NULL,platform VARCHAR(32),platform_account_id VARCHAR(255),browser_profile_id BIGINT,identification_status VARCHAR(32),duplicate_of_account_id BIGINT,platform_login_status VARCHAR(32),last_check_status VARCHAR(32),last_check_message VARCHAR(500),check_operation_id VARCHAR(64),check_profile_id BIGINT,check_started_at DATETIME,active_cookie TEXT,CONSTRAINT ` + fk + ` FOREIGN KEY (duplicate_of_account_id) REFERENCES ` + accounts + `(id),CONSTRAINT ` + check + ` CHECK (identification_status IN ('pending_identification','identified','duplicate')),UNIQUE KEY uq_media_accounts_user_platform_identity(user_id,platform,platform_account_id),UNIQUE KEY uq_media_accounts_profile_platform(browser_profile_id,platform))`)
	defer db.Exec("DROP TABLE " + accounts)
	exec(`CREATE TABLE ` + batches + ` (id BIGINT PRIMARY KEY,content_json JSON,revision BIGINT)`)
	defer db.Exec("DROP TABLE " + batches)
	exec(`INSERT INTO ` + accounts + `(id,user_id,platform,platform_account_id,browser_profile_id,identification_status,platform_login_status,last_check_status,active_cookie) VALUES (1,7,'bilibili','shared',21,'identified','normal','completed','keep-first-cookie'),(2,7,'bilibili',NULL,22,'duplicate','account_mismatch','completed','keep-second-cookie')`)
	exec(`UPDATE ` + accounts + ` SET duplicate_of_account_id=1 WHERE id=2`)
	raw := `{"remark":"keep-batch","items":[{"row_number":9,"status":"succeeded","account_id":"1"},{"row_number":3,"status":"failed","existing_account_id":"1","step":"applying","attempt":1,"operation_id":"import:4:3:1","input_cookie":"fixture-input","before_cookie":[],"fingerprint":"fixture-fingerprint"},{"row_number":5,"status":"skipped","previous_status":"failed","existing_account_id":"1","step":"applying"}]}`
	exec(`INSERT INTO `+batches+` VALUES(4,?,8)`, raw)
	content, err := os.ReadFile("../../../../migrations/20261031_086_same_account_multiple_windows.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.NewReplacer("fk_media_accounts_duplicate", fk, "chk_media_accounts_identification", check).Replace(string(content))
	sql = regexp.MustCompile(`\bmedia_accounts\b`).ReplaceAllString(sql, accounts)
	sql = regexp.MustCompile(`\bmedia_account_import_batches\b`).ReplaceAllString(sql, batches)
	// Remove comments before the controlled SQL statements are executed.
	var lines []string
	for _, line := range strings.Split(sql, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	for _, stmt := range strings.Split(strings.Join(lines, "\n"), ";") {
		if strings.TrimSpace(stmt) != "" {
			exec(stmt)
		}
	}
	var idStatus, login, cookie string
	if e = queryRow(db, `SELECT identification_status,platform_login_status,active_cookie FROM `+accounts+` WHERE id=2`).Scan(&idStatus, &login, &cookie); e != nil || idStatus != "pending_identification" || login != "unknown" || cookie != "keep-second-cookie" {
		t.Fatalf("legacy state was not safely reset: %v %s %s", e, idStatus, login)
	}
	// A newly verified shared UID can settle on the second window; same-window duplication cannot.
	exec(`UPDATE ` + accounts + ` SET platform_account_id='shared',identification_status='identified',platform_login_status='normal' WHERE id=2`)
	if err := db.Exec(`INSERT INTO ` + accounts + `(id,user_id,platform,platform_account_id,browser_profile_id,identification_status) VALUES(3,7,'bilibili','shared',22,'identified')`).Error; !duplicateKey(err) {
		t.Fatal("same-window/platform uniqueness was lost")
	}
	var updated []byte
	var revision int
	if e = queryRow(db, `SELECT content_json,revision FROM `+batches+` WHERE id=4`).Scan(&updated, &revision); e != nil {
		t.Fatal(e)
	}
	var batch struct {
		Remark string           `json:"remark"`
		Items  []map[string]any `json:"items"`
	}
	if e = json.Unmarshal(updated, &batch); e != nil {
		t.Fatal(e)
	}
	if revision != 9 || batch.Remark != "keep-batch" || len(batch.Items) != 3 || batch.Items[0]["row_number"] != float64(9) || batch.Items[1]["row_number"] != float64(3) || batch.Items[2]["row_number"] != float64(5) {
		t.Fatal("batch revision/order/metadata changed incorrectly")
	}
	item := batch.Items[1]
	if item["status"] != "uncertain" || item["step"] != "applying" || item["input_cookie"] != "fixture-input" || item["operation_id"] != "import:4:3:1" || item["same_identity_account_id"] != "1" || item["existing_account_id"] != nil {
		t.Fatal("legacy item cannot be inspected without replay")
	}
	if batch.Items[2]["status"] != "skipped" || batch.Items[2]["previous_status"] != "uncertain" {
		t.Fatal("legacy skipped row would replay after restore")
	}
}
