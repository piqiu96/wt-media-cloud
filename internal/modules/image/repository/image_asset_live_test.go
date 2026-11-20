package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database/migration"
	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	filerepo "github.com/wt-media/wt-media-cloud/internal/modules/files/repository"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/image/service"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func isolated(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("WT_MEDIA_TEST_LIVE_DB") != "1" {
		t.Skip("requires local MySQL; only a disposable schema is modified")
	}
	config.MustInitializeRuntimePaths()
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	c := cfg.Databases[0]
	d := mysql.NewConfig()
	d.User, d.Passwd, d.Net, d.Addr, d.ParseTime, d.Loc = c.Username, c.Password, "tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), true, time.UTC
	d.Params = map[string]string{"charset": "utf8mb4"}
	root, e := gorm.Open(gormmysql.Open(d.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("wt_image_test_%d", time.Now().UnixNano())
	if e = root.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci").Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Exec("DROP DATABASE " + name); raw, _ := root.DB(); raw.Close() })
	d.DBName = name
	db, e := gorm.Open(gormmysql.Open(d.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { raw, _ := db.DB(); raw.Close() })
	migrations, e := migration.LoadDir(config.GetRuntimePaths().Migrations)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = migration.Apply(context.Background(), db, migrations); e != nil {
		t.Fatal(e)
	}
	if replay, e := migration.Apply(context.Background(), db, migrations); e != nil || len(replay) != 0 {
		t.Fatalf("full migration replay: %v %v", replay, e)
	}
	if e = db.Exec("INSERT INTO operation_teams(id,name,created_at,updated_at) VALUES(10,'test team',NOW(6),NOW(6)),(20,'other team',NOW(6),NOW(6))").Error; e != nil {
		t.Fatal(e)
	}
	return db
}
func seedFile(t *testing.T, db *gorm.DB, id int64, width, height int) {
	t.Helper()
	metadata := fmt.Sprintf(`{"width":%d,"height":%d,"format":"png","source":"upload"}`, width, height)
	e := db.Exec(`INSERT INTO operations_files(id,owner_id,team_id,name,extension,type,content_type,object_key,size_bytes,sha256,resource_token,uploaded_at,created_at,updated_at,metadata_json) VALUES(?,1,10,'test','png','image','image/png','object',128,?,?,NOW(6),NOW(6),NOW(6),?)`, id, strings.Repeat("a", 64), fmt.Sprintf("%064d", id), metadata).Error
	if e != nil {
		t.Fatal(e)
	}
}
func fixture(db *gorm.DB) (*service.Service, identity.PublicUser) {
	team := identity.TeamID(10)
	a := identity.PublicUser{ID: 1, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{2}}
	s := &service.Service{Repo: repository.Store{DB: db}, ReadFile: func(ctx context.Context, a identity.PublicUser, id int64) (filemodel.File, error) {
		return (&files.Service{Repo: filerepo.Store{DB: db}}).Read(a, id)
	}, CategoryAllowed: func(a identity.PublicUser, id int64) (bool, error) { return id == 2 || id == 9, nil }}
	return s, a
}
func TestLiveImageRegistrationScopeSelectionAndFileProtection(t *testing.T) {
	db := isolated(t)
	s, a := fixture(db)
	seedFile(t, db, 7, 640, 360)
	seedFile(t, db, 8, 512, 512)
	in := service.RegisterInput{FileID: 7, Name: "测试封面", AllowAsCover: true, AllowAsBackground: true, CategoryIDs: []string{"2"}}
	v, e := s.RegisterImageAsset(context.Background(), a, in)
	if e != nil || v.Width != 640 || v.Height != 360 || v.ResourcePath == "" {
		t.Fatalf("registration %+v %v", v, e)
	}
	if again, e := s.RegisterImageAsset(context.Background(), a, in); e != nil || again.ID != v.ID {
		t.Fatalf("idempotence %+v %v", again, e)
	}
	in.FileID = 8
	in.CategoryIDs = []string{}
	general, e := s.RegisterImageAsset(context.Background(), a, in)
	if e != nil {
		t.Fatal(e)
	}
	rows, total, e := s.ListImageAsset(a, repository.Filter{Page: 1, PageSize: 20, Aspect: "landscape"})
	if e != nil || total != 1 || len(rows) != 1 || rows[0].ID != v.ID {
		t.Fatalf("dimensions filter %+v %d %v", rows, total, e)
	}
	other := a
	other.ID = 2
	rows, total, e = s.ListImageAsset(other, repository.Filter{Page: 1, PageSize: 20})
	if e != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("owner scope %+v %d %v", rows, total, e)
	}
	other.Role = identity.RoleSeniorOperator
	rows, total, e = s.ListImageAsset(other, repository.Filter{Page: 1, PageSize: 20})
	if e != nil || total != 2 {
		t.Fatalf("team scope %+v %d %v", rows, total, e)
	}
	other.CategoryIDs = nil
	rows, total, e = s.ListImageAsset(other, repository.Filter{Page: 1, PageSize: 20})
	if e != nil || total != 1 || rows[0].ID != general.ID {
		t.Fatalf("category scope %+v %d %v", rows, total, e)
	}
	selected, e := s.SelectImageAsset(a, service.SelectInput{Usage: "background", CategoryID: "2"})
	if e != nil || selected == nil || selected.ID != v.ID {
		t.Fatalf("category selection %+v %v", selected, e)
	}
	disabled := false
	v, e = s.SetImageAssetUsage(a, v.ID, service.UsageInput{ExpectedRevision: v.Revision, Enabled: &disabled})
	if e != nil {
		t.Fatal(e)
	}
	selected, e = s.SelectImageAsset(a, service.SelectInput{Usage: "background", CategoryID: "2"})
	if e != nil || selected == nil || selected.ID != general.ID {
		t.Fatalf("general fallback %+v %v", selected, e)
	}
	selected, e = s.SelectImageAsset(a, service.SelectInput{Usage: "background", CategoryID: "2", MinWidth: 1024})
	if e != nil || selected != nil {
		t.Fatalf("oversized candidate %+v %v", selected, e)
	}
	checker := filerepo.Store{DB: db, CheckReferences: service.CheckFileReferences}
	if e = checker.BeginDelete(7); !errors.Is(e, files.ErrInUse) {
		t.Fatalf("registered file deletion: %v", e)
	}
	var file filemodel.File
	if e = db.First(&file, 7).Error; e != nil || file.DeletedAt != nil {
		t.Fatalf("referenced file tombstoned %+v %v", file, e)
	}
	if e = db.Model(&model.ImageAsset{}).Where("id=?", general.ID).Update("team_id", 20).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.GetImageAsset(other, general.ID); !errors.Is(e, service.ErrNotFound) {
		t.Fatalf("cross-team access: %v", e)
	}
}
func TestLiveImageOptimisticUpdateAllowsOnlyOneWriter(t *testing.T) {
	db := isolated(t)
	s, a := fixture(db)
	seedFile(t, db, 7, 640, 360)
	v, e := s.RegisterImageAsset(context.Background(), a, service.RegisterInput{FileID: 7, Name: "封面", AllowAsCover: true, CategoryIDs: []string{}})
	if e != nil {
		t.Fatal(e)
	}
	repo := repository.Store{DB: db}
	errorsCh := make(chan error, 2)
	var group sync.WaitGroup
	for _, name := range []string{"first", "second"} {
		group.Add(1)
		go func(name string) {
			defer group.Done()
			changed := v
			changed.Name = name
			_, err := repo.Update(changed, 1)
			errorsCh <- err
		}(name)
	}
	group.Wait()
	close(errorsCh)
	success, conflict := 0, 0
	for err := range errorsCh {
		if err == nil {
			success++
		} else if errors.Is(err, repository.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("writers: success=%d conflict=%d", success, conflict)
	}
	current, e := s.GetImageAsset(a, v.ID)
	if e != nil || current.Revision != 2 {
		t.Fatalf("revision %+v %v", current, e)
	}
}

func TestLiveFileReadyPersistsInspectedMetadata(t *testing.T) {
	db := isolated(t)
	repo := filerepo.Store{DB: db}
	now := time.Now().UTC()
	team := int64(10)
	f := filemodel.File{OwnerID: 1, TeamID: &team, Name: "尺寸验证", Extension: "png", Type: "image", ContentType: "image/png", ResourceToken: strings.Repeat("f", 64), CreatedAt: now, UpdatedAt: now}
	if e := repo.Insert(&f); e != nil {
		t.Fatal(e)
	}
	f.MetadataJSON = []byte(`{"width":320,"height":240,"format":"png","source":"upload"}`)
	f.UploadedAt = &now
	if e := repo.Ready(f); e != nil {
		t.Fatal(e)
	}
	saved, e := repo.Find(f.ID)
	if e != nil || !strings.Contains(string(saved.MetadataJSON), `320`) {
		t.Fatalf("actual metadata missing: %s %v", saved.MetadataJSON, e)
	}
}

func TestLiveImageRegistrationAndFileDeletionCannotBothSucceed(t *testing.T) {
	db := isolated(t)
	s, a := fixture(db)
	checker := filerepo.Store{DB: db, CheckReferences: service.CheckFileReferences}
	for id := int64(101); id <= 105; id++ {
		seedFile(t, db, id, 640, 360)
		start := make(chan struct{})
		registered := make(chan error, 1)
		deleted := make(chan error, 1)
		go func(id int64) {
			<-start
			_, e := s.RegisterImageAsset(context.Background(), a, service.RegisterInput{FileID: id, Name: "并发封面", AllowAsCover: true, CategoryIDs: []string{}})
			registered <- e
		}(id)
		go func(id int64) { <-start; deleted <- checker.BeginDelete(id) }(id)
		close(start)
		registerErr, deleteErr := <-registered, <-deleted
		if registerErr == nil && deleteErr == nil {
			t.Fatal("file deletion and image registration both succeeded")
		}
		if registerErr != nil && deleteErr != nil {
			t.Fatalf("neither operation succeeded: %v / %v", registerErr, deleteErr)
		}
		var count int64
		if e := db.Model(&model.ImageAsset{}).Where("file_id=?", id).Count(&count).Error; e != nil {
			t.Fatal(e)
		}
		var f filemodel.File
		if e := db.First(&f, id).Error; e != nil {
			t.Fatal(e)
		}
		if registerErr == nil && (count != 1 || f.DeletedAt != nil) {
			t.Fatalf("registered reference was deleted: %+v", f)
		}
		if deleteErr == nil && (count != 0 || f.DeletedAt == nil) {
			t.Fatalf("deleted file was registered: %+v", f)
		}
	}
}
