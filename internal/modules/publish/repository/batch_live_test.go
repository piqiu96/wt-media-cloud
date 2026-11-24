package repository_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	filemodel "github.com/wt-media/wt-media-cloud/internal/modules/files/model"
	files "github.com/wt-media/wt-media-cloud/internal/modules/files/service"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	imagemodel "github.com/wt-media/wt-media-cloud/internal/modules/image/model"
	imagerepo "github.com/wt-media/wt-media-cloud/internal/modules/image/repository"
	accounts "github.com/wt-media/wt-media-cloud/internal/modules/mediaaccount/service"
	outputmodel "github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	repo "github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"gorm.io/gorm"
)

func TestBatchConfigurationPreservesOnlyCurrentFileReferences(t *testing.T) {
	db := isolated(t)
	s := store(db)
	// Retained compatible quick-task records provide a nonzero occupancy baseline.
	legacy, err := s.Create(task("legacy", 42, 1))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, TeamID: &team, CategoryIDs: []identity.CategoryID{1}, Role: identity.RoleOperator, Status: identity.UserStatusEnabled}
	providerDown := false
	svc := service.Service{
		BatchRepo: s, Repo: s,
		BatchCategory: func(id identity.CategoryID) (identity.Category, bool, error) {
			return identity.Category{ID: id, Enabled: true}, true, nil
		},
		Account: func(_ identity.PublicUser, id string) (accounts.Account, error) {
			return accounts.Account{ID: id, UserID: 9, TeamID: &team, CategoryIDs: []identity.CategoryID{1}, Platform: "baidu", Enabled: true, IdentificationStatus: accounts.IdentificationIdentified, PlatformLoginStatus: accounts.PlatformLoginNormal, PlatformAccountID: "fixture", BrowserProfileID: "1"}, nil
		},
		Output: func(_ identity.PublicUser, id int64) (production.ComposeOutputDetail, error) {
			if providerDown {
				return production.ComposeOutputDetail{}, fmt.Errorf("test provider unavailable")
			}
			return production.ComposeOutputDetail{Output: outputmodel.CompositeOutput{ID: id, TeamID: 7, CategoryID: 1, Platforms: []string{"baidu"}, Video: outputmodel.ComposeOutputVideo{Status: "ready"}, FileSizeBytes: 1, SHA256: strings.Repeat("b", 64)}}, nil
		},
		File: func(_ context.Context, _ identity.PublicUser, id, _ int64) (filemodel.File, error) {
			return filemodel.File{ID: id, Type: "image", SizeBytes: 1, SHA256: strings.Repeat("a", 64), MetadataJSON: []byte(`{"width":160,"height":90}`)}, nil
		},
	}
	rows := make([]model.BatchCandidate, 60)
	for i := range rows {
		rows[i] = model.BatchCandidate{RowKey: fmt.Sprintf("row-%d", i), InputRevision: "1", FieldRevisions: map[string]string{}, BatchPair: model.BatchPair{CategoryID: 1, MediaAccountID: 12, CompositeOutputID: int64(100 + i)}, Order: i, Content: model.Content{CoverFileID: 2}}
	}
	in := service.CreateBatchInput{
		RequestKey: "same", Name: "批次",
		BatchInput: service.BatchInput{Platform: "baidu", Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 60}}, Config: model.BatchConfig{Candidates: rows}, ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 5, ExceptionTaskLimit: 1, BetweenGroupPolicy: "auto_continue"}},
	}
	created, err := svc.CreateBatch(ctx, a, in)
	if err != nil || len(created.Config.Candidates) != 60 || created.Readiness.Ready || len(created.Readiness.Issues) == 0 || created.Readiness.Shortfalls[0].Shortfall != 0 {
		t.Fatalf("incomplete candidate save = id %d, rows %d, readiness %+v, err %v", created.ID, len(created.Config.Candidates), created.Readiness, err)
	}
	providerDown = true
	same, err := svc.CreateBatch(ctx, a, in)
	if err != nil || same.ID != created.ID || same.Revision != created.Revision {
		t.Fatalf("same-key recovery while provider fails: id %d revision %d, %v", same.ID, same.Revision, err)
	}
	different := in
	different.Name = "不同批次"
	if _, err = svc.CreateBatch(ctx, a, different); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("same key different content: %v", err)
	}
	providerDown = false

	noRead := a
	noRead.ID = 10
	filter := service.BatchFilter{Page: 1, PageSize: 1, RequestKey: in.RequestKey}
	for _, tc := range []struct {
		actor identity.PublicUser
		total int64
	}{{a, 1}, {noRead, 0}} {
		page, err := svc.ListBatches(tc.actor, filter)
		if err != nil || page.Total != tc.total || int64(len(page.Items)) != tc.total {
			t.Fatalf("exact request lookup = %+v, %v", page, err)
		}
	}
	if _, err = svc.GetBatch(noRead, created.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("foreign detail: %v", err)
	}
	manager := a
	manager.ID, manager.Role = 10, identity.RoleSeniorOperator
	page, err := svc.ListBatches(manager, service.BatchFilter{Page: 1, PageSize: 1})
	if err != nil || page.Total != 1 || len(page.Items[0].AllowedActions) != 0 {
		t.Fatalf("manager scoped list: %+v %v", page, err)
	}
	manager.CategoryIDs = []identity.CategoryID{2}
	page, err = svc.ListBatches(manager, service.BatchFilter{Page: 1, PageSize: 1})
	if err != nil || page.Total != 0 {
		t.Fatalf("out-of-category manager list: %+v %v", page, err)
	}

	if err := fileStore(db).BeginDelete(2); !errors.Is(err, files.ErrInUse) {
		t.Fatalf("saved file must be protected: %v", err)
	}
	edited := service.EditBatchInput{BatchInput: in.BatchInput, Name: in.Name, ExpectedRevision: 1}
	edited.Config.Candidates = append([]model.BatchCandidate{}, rows[:58]...)
	for i := range edited.Config.Candidates {
		edited.Config.Candidates[i].Content.CoverFileID = 3
	}
	changed, err := svc.EditBatch(ctx, a, created.ID, edited)
	if err != nil || changed.Revision != 2 || changed.Targets[0].TargetCount != 60 || changed.Readiness.Shortfalls[0].Shortfall != 2 || len(changed.Config.Candidates) != 58 {
		t.Fatalf("edit = revision %d, candidates %d, gaps %+v, err %v", changed.Revision, len(changed.Config.Candidates), changed.Readiness.Shortfalls, err)
	}
	for i, row := range changed.Config.Candidates {
		if row.RowKey != rows[i].RowKey {
			t.Fatalf("row %d changed identity: %s", i, row.RowKey)
		}
	}
	if _, err = svc.EditBatch(ctx, a, created.ID, edited); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("stale edit = %v", err)
	}
	if err := fileStore(db).BeginDelete(2); err != nil {
		t.Fatalf("removed file reference was retained: %v", err)
	}
	if err := fileStore(db).BeginDelete(3); !errors.Is(err, files.ErrInUse) {
		t.Fatalf("replacement file not protected: %v", err)
	}
	edited.ExpectedRevision = 2
	for i := range edited.Config.Candidates {
		edited.Config.Candidates[i].Content.CoverFileID = 2
	}
	if _, err = svc.EditBatch(ctx, a, created.ID, edited); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("edit to deleted file = %v", err)
	}
	records, err := svc.BatchRecords(a, created.ID, 1, 20)
	if err != nil || records.Total != 2 || len(records.Items) != 2 {
		t.Fatalf("audit = %+v, %v", records, err)
	}
	if stored, err := svc.GetBatch(a, created.ID); err != nil || stored.Revision != 2 || stored.Config.Candidates[0].Content.CoverFileID != 3 {
		t.Fatalf("save rollback lost current configuration: id=%d revision=%d err=%v", stored.ID, stored.Revision, err)
	}
	for table, want := range map[string]int64{"publication_task": 1, "publication_attempt": 0} {
		var count int64
		if err = db.Table(table).Count(&count).Error; err != nil || count != want {
			t.Fatalf("%s count=%d want %d: %v", table, count, want, err)
		}
	}
	occupied, err := s.OccupiedOutputIDs("baidu", append([]int64{42}, 100, 101, 159))
	if err != nil || len(occupied) != 1 || occupied[0] != 42 {
		t.Fatalf("occupancy = %v, %v", occupied, err)
	}
	if got, err := s.Get(legacy.ID); err != nil || got.ID != legacy.ID || got.Revision != legacy.Revision {
		t.Fatalf("old task changed = %+v, %v", got, err)
	}
	if _, err = svc.CopyBatch(ctx, a, created.ID, 2); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("not-started copy unexpectedly allowed: %v", err)
	}
	if err = db.Exec("UPDATE publication_batch SET execution_status='ended' WHERE id=?", created.ID).Error; err != nil {
		t.Fatal(err)
	}
	copied, err := svc.CopyBatch(ctx, a, created.ID, 2)
	if err != nil || copied.SourceID != created.ID || copied.SourceRevision != 2 || len(copied.Config.Candidates) != 58 ||
		copied.Config.Candidates[0].Content.Confirmed || copied.Config.Candidates[0].Content.CoverFileID != 0 {
		t.Fatalf("copy response: source=%d revision=%d rows=%d error=%v", copied.SourceID, copied.SourceRevision, len(copied.Config.Candidates), err)
	}
	if _, err = svc.EditBatch(ctx, a, created.ID, edited); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("ended configuration unexpectedly editable: %v", err)
	}

	// Exercise removal of the old configuration reference table in this disposable database.
	for _, table := range []string{"publication_batch_file"} {
		if err = db.Migrator().DropTable(table); err != nil {
			t.Fatalf("rollback %s: %v", table, err)
		}
	}
	if got, err := s.Get(legacy.ID); err != nil || got.ID != legacy.ID || got.Revision != legacy.Revision {
		t.Fatalf("rollback changed original quick task: id=%d revision=%d err=%v", got.ID, got.Revision, err)
	}

}

func TestBatchSaveAndDeleteSerializeOnFile(t *testing.T) {
	db := isolated(t)
	s := store(db)
	locked, release, started := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s.LockFiles = func(tx *gorm.DB, ids []int64) error {
		if err := files.LockAvailable(tx, ids); err != nil {
			return err
		}
		close(locked)
		<-release
		return nil
	}
	v := model.Batch{
		TeamID: 7, OperatorID: 9, CreatedBy: 9, Name: "race", Platform: "baidu", RequestKey: "race", RequestHash: "a", Revision: 1,
		ConfigurationStatus: model.BatchIncomplete, ExecutionStatus: model.BatchNotStarted,
		Targets:         []model.BatchTarget{{CategoryID: 1, TargetCount: 1}},
		Config:          model.BatchConfig{Candidates: []model.BatchCandidate{{Content: model.Content{CoverFileID: 2}}}},
		CandidatesCount: 1, ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 1, ExceptionTaskLimit: 1, BetweenGroupPolicy: "auto_continue"},
	}
	saved, deleted := make(chan error, 1), make(chan error, 1)
	go func() { _, err := s.CreateBatch(v); saved <- err }()
	select {
	case <-locked:
	case err := <-saved:
		t.Fatalf("writer failed to lock: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not lock reference")
	}
	go func() { close(started); deleted <- fileStore(db).BeginDelete(2) }()
	<-started
	close(release)
	if err := <-saved; err != nil {
		t.Fatal(err)
	}
	if err := <-deleted; !errors.Is(err, files.ErrInUse) {
		t.Fatalf("concurrent delete = %v", err)
	}

	if err := fileStore(db).BeginDelete(3); err != nil {
		t.Fatal(err)
	}
	v.RequestKey = "deleted-before-save"
	v.Config.Candidates[0].Content.CoverFileID = 3
	if _, err := s.CreateBatch(v); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("create after delete = %v", err)
	}
	if _, err := s.BatchByRequest(9, v.RequestKey); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("failed create left a batch: %v", err)
	}
}

func TestBatchFileReferenceBlocksConcurrentImageUnregisterAfterSnapshot(t *testing.T) {
	db := isolated(t)
	if err := db.Exec("CREATE TABLE operation_teams(id BIGINT UNSIGNED PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO operation_teams(id) VALUES(7)").Error; err != nil {
		t.Fatal(err)
	}
	applyFileMigration(t, db, "20261121_110_image_asset.sql")
	imageStore := imagerepo.Store{DB: db}
	asset, err := imageStore.Register(imagemodel.ImageAsset{FileID: 2, TeamID: 7, Name: "race", CreatedBy: 9, UpdatedBy: 9, AllowAsCover: true, Enabled: true, Revision: 1, CategoryIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}

	held, snapshot, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var observed sync.Once
	const callback = "test:batch_reference_delete_snapshot"
	if err = db.Callback().Row().After("gorm:row").Register(callback, func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "image_asset AS a") {
			observed.Do(func() { close(snapshot) })
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Row().Remove(callback)
	batchStore := store(db)
	batchStore.LockFiles = func(tx *gorm.DB, ids []int64) error {
		if err := files.LockAvailable(tx, ids); err != nil {
			return err
		}
		close(held)
		<-release
		return nil
	}
	v := model.Batch{TeamID: 7, OperatorID: 9, CreatedBy: 9, Name: "race", Platform: "baidu", RequestKey: "unregister-race", RequestHash: "a", Revision: 1,
		ConfigurationStatus: model.BatchIncomplete, ExecutionStatus: model.BatchNotStarted, Targets: []model.BatchTarget{{CategoryID: 1, TargetCount: 1}},
		Config:          model.BatchConfig{Candidates: []model.BatchCandidate{{Content: model.Content{CoverFileID: 2}}}},
		CandidatesCount: 1, ExecutionSetting: model.BatchExecutionSetting{ConcurrencyLimit: 1, ExceptionTaskLimit: 1, BetweenGroupPolicy: "auto_continue"}}
	saved, deleted := make(chan error, 1), make(chan error, 1)
	go func() { _, err := batchStore.CreateBatch(v); saved <- err }()
	select {
	case <-held:
	case err := <-saved:
		t.Fatalf("batch create did not lock: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("batch writer did not hold file")
	}
	go func() {
		deleted <- imageStore.Delete(asset.ID, asset.Revision, service.CheckFileReferences)
	}()
	select {
	case <-snapshot:
		close(release)
	case err := <-deleted:
		close(release)
		t.Fatalf("image unregister returned before snapshot: %v", err)
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("image unregister did not establish snapshot")
	}
	if err := <-saved; err != nil {
		t.Fatal(err)
	}
	if err := <-deleted; !errors.Is(err, files.ErrInUse) {
		t.Fatalf("concurrent unregister should retain library registration: %v", err)
	}
	if _, err = imageStore.Get(asset.ID); err != nil {
		t.Fatalf("registration missing: %v", err)
	}
}
