package repository_test

import (
	"context"
	"errors"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database/migration"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	filesrepo "github.com/wt-media/wt-media-cloud/internal/modules/files/repository"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	productionrepo "github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/publishing/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publishing/repository"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"sync"
	"testing"
	"time"
)

// This creates an isolated, disposable schema, never test task rows in the live queue.
func isolated(t *testing.T) *gorm.DB { return isolatedDB(t, true) }
func isolatedDB(t *testing.T, fixtures bool) *gorm.DB {
	t.Helper()
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires local MySQL; creates and removes an isolated test schema")
	}
	config.MustInitializeRuntimePaths()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := cfg.Databases[0]
	dsn := driver.NewConfig()
	dsn.User = c.Username
	dsn.Passwd = c.Password
	dsn.Net = "tcp"
	dsn.Addr = fmt.Sprintf("%s:%d", c.Host, c.Port)
	dsn.ParseTime = true
	dsn.Loc = time.UTC
	dsn.Params = map[string]string{"charset": "utf8mb4"}
	root, err := gorm.Open(gormmysql.Open(dsn.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("wt_pub_test_%d", time.Now().UnixNano())
	if err = root.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci").Error; err != nil {
		t.Fatal(err)
	}
	dsn.DBName = name
	db, err := gorm.Open(gormmysql.Open(dsn.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sql, _ := db.DB()
		sql.Close()
		root.Exec("DROP DATABASE " + name)
		sql, _ = root.DB()
		sql.Close()
	})
	if !fixtures {
		return db
	}
	exec := func(q string) {
		t.Helper()
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Reference ID types match current provider migrations. No foreign key checks are disabled.
	exec("CREATE TABLE media_accounts(id BIGINT UNSIGNED PRIMARY KEY)")
	exec("CREATE TABLE composite_outputs(id BIGINT UNSIGNED PRIMARY KEY)")
	exec("CREATE TABLE audit_logs LIKE " + c.Database + ".audit_logs")
	body, err := os.ReadFile(config.GetRuntimePaths().Migrations + "/20261106_098_operations_files.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = migration.Apply(context.Background(), db, []migration.Migration{{Version: "base_files", Name: "files", SQL: string(body)}}); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(config.GetRuntimePaths().Migrations + "/20261118_102_publication_task.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrations := []migration.Migration{{Version: "publication", Name: "publication", SQL: string(body)}}
	if result, err := migration.Apply(context.Background(), db, migrations); err != nil || len(result) != 1 {
		t.Fatalf("migration result=%v err=%v", result, err)
	}
	if result, err := migration.Apply(context.Background(), db, migrations); err != nil || len(result) != 0 {
		t.Fatalf("migration replay result=%v err=%v", result, err)
	}
	exec("INSERT INTO media_accounts(id) VALUES(12)")
	exec("INSERT INTO composite_outputs(id) VALUES(42),(43),(44)")
	for id := 1; id <= 3; id++ {
		if err = db.Exec(`INSERT INTO operations_files(id,owner_id,name,extension,type,content_type,object_key,size_bytes,sha256,resource_token,uploaded_at,created_at,updated_at) VALUES(?,9,'cover','png','image','image/png',?,3,?,?,NOW(6),NOW(6),NOW(6))`, id, fmt.Sprint("cover-", id), fmt.Sprintf("%064d", id), fmt.Sprintf("%064d", id)).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}
func task(key string, output int64, file int64) model.Task {
	now := time.Now().UTC()
	return model.Task{TeamID: 7, OperatorID: 9, CategoryID: 1, MediaAccountID: 12, CompositeOutputID: output, Platform: "baidu", PlatformAccountUID: "exact-UID", CoverFileID: file, ResourceSnapshot: []byte(`{"cover_name":"cover"}`), Content: model.Content{Description: "数据库并发测试，不会执行投稿", CoverFileID: file, Declaration: "无需声明", Confirmed: true}, Status: "pending", Revision: 1, CreatedBy: 9, RequestKey: key, RequestHash: fmt.Sprintf("%064d", 1), CreatedAt: now, UpdatedAt: now}
}
func store(db *gorm.DB) repo.Store { return repo.Store{DB: db, LockFiles: files.LockAvailable} }
func fileStore(db *gorm.DB) filesrepo.Store {
	return filesrepo.Store{DB: db, CheckReferences: func(tx *gorm.DB, id int64) error {
		used, err := repo.FileReferenced(tx, id)
		if err != nil {
			return err
		}
		if used {
			return filesrepo.ErrInUse
		}
		return nil
	}}
}
func TestLivePublicationCreateRaceAndAudit(t *testing.T) {
	db := isolated(t)
	s := store(db)
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := s.Create(task(fmt.Sprint("request-", n), 42, 1))
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success, occupied := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, repo.ErrOccupied) {
			occupied++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || occupied != 11 {
		t.Fatalf("success=%d occupied=%d", success, occupied)
	}
	var tasks, audits int64
	db.Model(&model.Task{}).Count(&tasks)
	db.Table("audit_logs").Where("target_type='publication'").Count(&audits)
	if tasks != 1 || audits != 1 {
		t.Fatalf("tasks=%d audits=%d", tasks, audits)
	}
	var v model.Task
	db.First(&v)
	again, err := s.Create(v)
	if err != nil || again.ID != v.ID {
		t.Fatalf("idempotent retry: %v %+v", err, again)
	}
	v.RequestHash = fmt.Sprintf("%064d", 2)
	if _, err := s.Create(v); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("same key changed content: %v", err)
	}
	if err := fileStore(db).BeginDelete(1); !errors.Is(err, filesrepo.ErrInUse) {
		t.Fatalf("referenced file deletable: %v", err)
	}
}
func TestLiveReferenceCreationAndDeleteSerialize(t *testing.T) {
	db := isolated(t)
	locked, release := make(chan struct{}), make(chan struct{})
	s := store(db)
	s.LockFiles = func(tx *gorm.DB, ids []int64) error {
		if err := files.LockAvailable(tx, ids); err != nil {
			return err
		}
		close(locked)
		<-release
		return nil
	}
	created, deleted := make(chan error, 1), make(chan error, 1)
	go func() { _, err := s.Create(task("racing-reference", 43, 2)); created <- err }()
	<-locked
	go func() { deleted <- fileStore(db).BeginDelete(2) }()
	close(release)
	if err := <-created; err != nil {
		t.Fatal(err)
	}
	if err := <-deleted; !errors.Is(err, filesrepo.ErrInUse) {
		t.Fatalf("delete won against saved reference: %v", err)
	}
	if err := fileStore(db).BeginDelete(3); err != nil {
		t.Fatal(err)
	}
	if _, err := store(db).Create(task("delete-first", 44, 3)); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("reference created after deleting marker: %v", err)
	}
	if err := fileStore(db).BeginDelete(3); err != nil {
		t.Fatalf("deletion retry: %v", err)
	}
	if err := fileStore(db).Delete(3); err != nil {
		t.Fatal(err)
	}
}
func TestLiveEditRevisionAndAuditRollback(t *testing.T) {
	db := isolated(t)
	s := store(db)
	v, err := s.Create(task("edit", 42, 1))
	if err != nil {
		t.Fatal(err)
	}
	v.AuditActor = 10
	v.Content.Description = "新文案"
	edited, err := s.Edit(v, 1)
	if err != nil || edited.Revision != 2 || edited.CreatedBy != 9 {
		t.Fatalf("edit=%+v err=%v", edited, err)
	}
	if _, err = s.Edit(v, 1); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("stale edit=%v", err)
	}
	if err = db.Exec("DROP TABLE audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	edited.Content.Description = "must rollback"
	if _, err = s.Edit(edited, 2); err == nil {
		t.Fatal("audit error ignored")
	}
	got, err := s.Get(v.ID)
	if err != nil || got.Revision != 2 || got.Content.Description != "新文案" {
		t.Fatalf("audit failure overwrote task=%+v err=%v", got, err)
	}
}

func TestLiveSavedProductionJSONReferenceAndDelete(t *testing.T) {
	db := isolated(t)
	for _, q := range []string{
		"CREATE TABLE compose_strategies(id BIGINT PRIMARY KEY, config_json JSON,output_settings_json JSON)",
		"CREATE TABLE compose_tasks(id BIGINT PRIMARY KEY,config_snapshot_json JSON,execution_manifest_json JSON,technical_config_json JSON)",
		"CREATE TABLE compose_task_batches(id BIGINT PRIMARY KEY,config_snapshot_json JSON)",
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := fmt.Sprintf("%064d", 1)
	fs := filesrepo.Store{DB: db, CheckReferences: func(tx *gorm.DB, id int64) error {
		used, err := productionrepo.OperationsFileReferenced(tx, token)
		if err != nil {
			return err
		}
		if used {
			return files.ErrInUse
		}
		return nil
	}}
	// An actual writer holds the same file lock through its JSON insert.
	locked, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		finished <- db.Transaction(func(tx *gorm.DB) error {
			raw := []byte(fmt.Sprintf(`{"intro_video_url":"http://localhost/api/v1/file-resources/%s?preview=1"}`, token))
			if err := files.LockJSONReferences(tx, raw); err != nil {
				return err
			}
			close(locked)
			<-release
			return tx.Exec("INSERT INTO compose_strategies(id,config_json) VALUES(1,?)", string(raw)).Error
		})
	}()
	<-locked
	deleting := make(chan error, 1)
	go func() { deleting <- fs.BeginDelete(1) }()
	select {
	case err := <-deleting:
		t.Fatalf("deletion bypassed writer lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if err := <-deleting; !errors.Is(err, files.ErrInUse) {
		t.Fatalf("saved reference lost: %v", err)
	}
	// Once deletion is marked first, a JSON writer is rejected, not silently saved.
	fs.CheckReferences = func(*gorm.DB, int64) error { return nil }
	if err := fs.BeginDelete(2); err != nil {
		t.Fatal(err)
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		return files.LockJSONReferences(tx, []byte(fmt.Sprintf(`{"cover":"/api/v1/file-resources/%064d"}`, 2)))
	})
	if !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("deleting file referenced: %v", err)
	}
}

func TestLiveFullMigrationChain(t *testing.T) {
	db := isolatedDB(t, false)
	all, err := migration.LoadDir(config.GetRuntimePaths().Migrations)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := migration.Apply(context.Background(), db, all)
	if err != nil || len(applied) != len(all) {
		t.Fatalf("fresh migration chain applied=%d/%d err=%v", len(applied), len(all), err)
	}
	replay, err := migration.Apply(context.Background(), db, all)
	if err != nil || len(replay) != 0 {
		t.Fatalf("migration replay applied=%d err=%v", len(replay), err)
	}
	t.Logf("fresh schema and replay: %d migrations", len(all))
}
func TestLiveTaskListFiltersScopeAndRealSummary(t *testing.T) {
	db := isolated(t)
	s := store(db)
	for i := int64(0); i < 3; i++ {
		v := task(fmt.Sprint("list-", i), 42+i, 1)
		if i == 1 {
			v.OperatorID = 10
		}
		if i == 2 {
			v.OperatorID = 10
			v.TeamID = 8
		}
		if _, err := s.Create(v); err != nil {
			t.Fatal(err)
		}
	}
	team := identity.TeamID(7)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	f := repo.Filter{Actor: actor, Page: 1, PageSize: 1}
	items, total, err := s.List(f)
	if err != nil || total != 1 || len(items) != 1 || items[0].OperatorID != 9 || items[0].TeamID != 7 {
		t.Fatalf("operator scope %v %d %v", items, total, err)
	}
	f.Actor.Role = identity.RoleSeniorOperator
	items, total, err = s.List(f)
	if err != nil || total != 2 || len(items) != 1 {
		t.Fatalf("senior pagination total=%d items=%d err=%v", total, len(items), err)
	}
	f.Status = "running"
	counts, err := s.Summary(f)
	if err != nil || counts["all"] != 2 || counts["running"] != 0 {
		t.Fatalf("summary used page/status or leaked team: %v %v", counts, err)
	}
	f.Platform = "bilibili"
	_, total, err = s.List(f)
	if err != nil || total != 0 {
		t.Fatalf("platform filter total=%d err=%v", total, err)
	}
	f.Platform = ""
	f.Status = ""
	f.CategoryID = 2
	_, total, err = s.List(f)
	if err != nil || total != 0 {
		t.Fatalf("category filter total=%d err=%v", total, err)
	}
	f.CategoryID = 0
	future := time.Now().UTC().Add(time.Hour)
	f.CreatedFrom = &future
	_, total, err = s.List(f)
	if err != nil || total != 0 {
		t.Fatalf("date filter total=%d err=%v", total, err)
	}
}

// Upload constructs a File without a deletion state. GORM must preserve the
// database's ready default instead of writing an empty string that listing hides.
func TestLiveUploadedFileDefaultsToReadyAndCanBeReferenced(t *testing.T) {
	db := isolated(t)
	now := time.Now().UTC()
	f := filemodel.File{OwnerID: 9, Name: "new-upload", Extension: "png", Type: "image", ContentType: "image/png", ObjectKey: "fresh-upload", SizeBytes: 3, SHA256: fmt.Sprintf("%064d", 8), ResourceToken: fmt.Sprintf("%064d", 8), UploadedAt: &now, CreatedAt: now, UpdatedAt: now}
	if err := (filesrepo.Store{DB: db}).Insert(&f); err != nil {
		t.Fatal(err)
	}
	var saved filemodel.File
	if err := db.First(&saved, f.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.DeleteState != "ready" {
		t.Fatalf("new upload persisted delete_state=%q", saved.DeleteState)
	}
	if _, err := store(db).Create(task("fresh-upload", 42, f.ID)); err != nil {
		t.Fatalf("reference fresh upload: %v", err)
	}
	if err := fileStore(db).BeginDelete(f.ID); !errors.Is(err, files.ErrInUse) {
		t.Fatalf("referenced fresh upload deletion: %v", err)
	}
}

func TestLiveFileStateMigrationRepairsBlankWithoutRevivingDeleting(t *testing.T) {
	db := isolated(t)
	if err := db.Exec("UPDATE operations_files SET delete_state='' WHERE id=1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE operations_files SET delete_state='deleting' WHERE id=2").Error; err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile(config.GetRuntimePaths().Migrations + "/20261118_103_operations_file_delete_state.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrations := []migration.Migration{{Version: "file_state_repair", Name: "file_state_repair", SQL: string(sql)}}
	if applied, err := migration.Apply(context.Background(), db, migrations); err != nil || len(applied) != 1 {
		t.Fatalf("repair applied=%v err=%v", applied, err)
	}
	for id, want := range map[int64]string{1: "ready", 2: "deleting", 3: "ready"} {
		var f filemodel.File
		if err := db.First(&f, id).Error; err != nil {
			t.Fatal(err)
		}
		if f.DeleteState != want {
			t.Fatalf("file %d state=%q want=%q", id, f.DeleteState, want)
		}
	}
	if applied, err := migration.Apply(context.Background(), db, migrations); err != nil || len(applied) != 0 {
		t.Fatalf("repair replay=%v err=%v", applied, err)
	}
	if err := db.Exec("UPDATE operations_files SET delete_state='' WHERE id=3").Error; err == nil {
		t.Fatal("empty state accepted after repair")
	}
}
