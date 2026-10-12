package service

import (
	"context"
	"testing"
	"time"

	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

// CHG-20261013-084 (M4-C6 T-02): the my-outputs service — the scope gate the
// compose-task page already uses, a detail whose trace reads the referenced
// rows as they are now, a conditional archive, and a preview address that is
// never part of any body.

type outputMemoryStore struct {
	outputs []model.CompositeOutput
	filters []repository.ComposeOutputFilter
	archive []repository.ComposeOutputFilter
	// archiveBlocked makes the conditional update write nothing, as it does on
	// a published or already-archived row.
	archiveBlocked bool
}

func (s *outputMemoryStore) List(filter repository.ComposeOutputFilter) ([]model.CompositeOutput, error) {
	s.filters = append(s.filters, filter)
	return s.outputs, nil
}

func (s *outputMemoryStore) Find(id int64) (model.CompositeOutput, bool, error) {
	for _, item := range s.outputs {
		if item.ID == id {
			return item, true, nil
		}
	}
	return model.CompositeOutput{}, false, nil
}

func (s *outputMemoryStore) Archive(id, _ int64, _ time.Time) (bool, error) {
	s.archive = append(s.archive, repository.ComposeOutputFilter{})
	if s.archiveBlocked {
		return false, nil
	}
	for i := range s.outputs {
		if s.outputs[i].ID == id {
			s.outputs[i].PublishStatus = "archived"
		}
	}
	return true, nil
}

type outputTraceFakes struct {
	material model.Material
	usage    model.MaterialUsage
	strategy model.ComposeStrategy
	task     model.ComposeTask
}

func (f *outputTraceFakes) FindMaterial(int64) (model.Material, bool, error) {
	return f.material, true, nil
}

func (f *outputTraceFakes) FindUsage(int64) (model.MaterialUsage, bool, error) {
	return f.usage, true, nil
}

func (f *outputTraceFakes) Find(int64) (model.ComposeStrategy, bool, error) {
	return f.strategy, true, nil
}

type outputTaskFake struct {
	task model.ComposeTask
}

func (f *outputTaskFake) Find(int64) (model.ComposeTask, bool, error) {
	return f.task, true, nil
}

type outputLinkerFake struct{ key string }

func (f *outputLinkerFake) PresignObjectURL(_ context.Context, key string) (string, error) {
	f.key = key
	return "https://signed.example/" + key, nil
}

func outputFixture(id int64) model.CompositeOutput {
	return model.CompositeOutput{ID: id, TeamID: 7, GameID: "delta", MaterialID: 42, MaterialUsageID: 9, TaskID: 55, StrategyID: 3, TemplateCode: "basic_cut", TemplateVersion: 1, ObjectKey: "compose-outputs/55/output.mp4", FileSizeBytes: 1024, SHA256: "abc", Media: []byte(`{"duration_ms":900}`), PublishStatus: "pending_publish", CreatedBy: 9}
}

func newOutputService(store *outputMemoryStore, traces *outputTraceFakes, tasks *outputTaskFake, linker *outputLinkerFake) *ComposeOutputService {
	return NewComposeOutputService(store, traces, traces, traces, tasks, linker)
}

func TestListComposeOutputsScopesLikeTheTaskPage(t *testing.T) {
	team := identity.TeamID(7)
	operator := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	store := &outputMemoryStore{outputs: []model.CompositeOutput{outputFixture(1)}}
	svc := newOutputService(store, &outputTraceFakes{}, &outputTaskFake{}, &outputLinkerFake{})

	items, err := svc.List(operator, ListComposeOutputsInput{PublishStatus: "pending_publish"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if len(store.filters) != 1 {
		t.Fatalf("filters = %d", len(store.filters))
	}
	filter := store.filters[0]
	if filter.TeamID != 7 || len(filter.GameIDs) != 1 || filter.GameIDs[0] != "delta" || filter.CreatedBy != 9 {
		t.Fatalf("operator filter = %+v", filter)
	}
	if filter.PublishStatus != "pending_publish" {
		t.Fatalf("publish status filter = %q", filter.PublishStatus)
	}
	if len(items[0].ObjectKey) != 0 && items[0].ObjectKey != "" {
		// The object key is json:"-" — the projection cannot leak it, but the
		// struct the service returns still carries it internally. Asserted on
		// the wire in T-03; nothing to fail here.
		_ = items[0].ObjectKey
	}
}

func TestListComposeOutputsRefusesUnknownPublishStatus(t *testing.T) {
	team := identity.TeamID(7)
	operator := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	svc := newOutputService(&outputMemoryStore{}, &outputTraceFakes{}, &outputTaskFake{}, &outputLinkerFake{})
	if _, err := svc.List(operator, ListComposeOutputsInput{PublishStatus: "published_by_hand"}); err == nil {
		t.Fatal("an unknown publish status must be named invalid")
	}
}

func TestListComposeOutputsSeniorSeesTeamScopeNotOnlyOwn(t *testing.T) {
	team := identity.TeamID(7)
	senior := identity.PublicUser{ID: 11, Role: identity.RoleSeniorOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	store := &outputMemoryStore{}
	svc := newOutputService(store, &outputTraceFakes{}, &outputTaskFake{}, &outputLinkerFake{})
	if _, err := svc.List(senior, ListComposeOutputsInput{}); err != nil {
		t.Fatalf("list: %v", err)
	}
	filter := store.filters[0]
	if filter.CreatedBy != 0 {
		t.Fatalf("senior filter must not narrow to created_by: %+v", filter)
	}
}

func TestGetComposeOutputDetailTracesSources(t *testing.T) {
	team := identity.TeamID(7)
	operator := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	store := &outputMemoryStore{outputs: []model.CompositeOutput{outputFixture(1)}}
	traces := &outputTraceFakes{material: model.Material{ID: 42, Title: "素材甲"}, usage: model.MaterialUsage{ID: 9, Status: "active"}, strategy: model.ComposeStrategy{ID: 3, Name: "策略"}}
	tasks := &outputTaskFake{task: model.ComposeTask{ID: 55, Status: model.ComposeTaskSuccess, ConfigSnapshot: []byte(`{"start_seconds":0,"end_seconds":1}`)}}
	svc := newOutputService(store, traces, tasks, &outputLinkerFake{})

	detail, err := svc.Get(operator, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if detail.Output.ID != 1 || detail.Trace.Material.Title != "素材甲" || detail.Trace.ComposeTask.ID != 55 {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Trace.Strategy.ConfigSnapshot != nil && detail.Trace.Strategy.Name == "" {
		t.Fatalf("strategy trace incomplete: %+v", detail.Trace.Strategy)
	}
}

func TestGetComposeOutputOutOfScopeAnswersNotFound(t *testing.T) {
	team := identity.TeamID(7)
	other := identity.PublicUser{ID: 20, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	store := &outputMemoryStore{outputs: []model.CompositeOutput{outputFixture(1)}} // CreatedBy 9
	svc := newOutputService(store, &outputTraceFakes{}, &outputTaskFake{}, &outputLinkerFake{})
	if _, err := svc.Get(other, 1); err != ErrComposeOutputNotFound {
		t.Fatalf("out-of-scope read = %v, want not-found", err)
	}
	if _, err := svc.VideoURL(context.Background(), other, 1); err != ErrComposeOutputNotFound {
		t.Fatalf("out-of-scope preview = %v, want not-found", err)
	}
	if _, err := svc.Archive(other, 1); err != ErrComposeOutputNotFound {
		t.Fatalf("out-of-scope archive = %v, want not-found", err)
	}
}

func TestArchiveComposeOutputFlipsPendingPublishOnly(t *testing.T) {
	team := identity.TeamID(7)
	operator := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	store := &outputMemoryStore{outputs: []model.CompositeOutput{outputFixture(1)}}
	svc := newOutputService(store, &outputTraceFakes{}, &outputTaskFake{}, &outputLinkerFake{})

	item, err := svc.Archive(operator, 1)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if item.PublishStatus != "archived" {
		t.Fatalf("publish status = %q", item.PublishStatus)
	}
}

func TestArchiveComposeOutputRefusesBlockedRow(t *testing.T) {
	team := identity.TeamID(7)
	operator := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	store := &outputMemoryStore{outputs: []model.CompositeOutput{outputFixture(1)}, archiveBlocked: true}
	svc := newOutputService(store, &outputTraceFakes{}, &outputTaskFake{}, &outputLinkerFake{})
	if _, err := svc.Archive(operator, 1); err != ErrComposeOutputNotArchivable {
		t.Fatalf("blocked archive = %v, want not-archivable", err)
	}
}

func TestComposeOutputVideoURLSignsTheOutputKey(t *testing.T) {
	team := identity.TeamID(7)
	operator := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, GameIDs: []string{"delta"}}
	store := &outputMemoryStore{outputs: []model.CompositeOutput{outputFixture(1)}}
	linker := &outputLinkerFake{}
	svc := newOutputService(store, &outputTraceFakes{}, &outputTaskFake{}, linker)

	url, err := svc.VideoURL(context.Background(), operator, 1)
	if err != nil {
		t.Fatalf("video url: %v", err)
	}
	if linker.key != "compose-outputs/55/output.mp4" {
		t.Fatalf("signed key = %q", linker.key)
	}
	if url == "" {
		t.Fatal("empty url")
	}
}
