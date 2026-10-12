package service

import (
	"errors"
	"testing"
	"time"

	identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"github.com/wt-media/wt-media-cloud/internal/shared/identity"
)

type strategyMemoryStore struct {
	rows     map[int64]model.ComposeStrategy
	versions map[int64][]model.ComposeStrategyVersion
	updates  int
}

func (s *strategyMemoryStore) CreateStrategy(row model.ComposeStrategy) (model.ComposeStrategy, error) {
	if s.rows == nil {
		s.rows = map[int64]model.ComposeStrategy{}
	}
	if s.versions == nil {
		s.versions = map[int64][]model.ComposeStrategyVersion{}
	}
	row.ID = int64(len(s.rows) + 1)
	s.rows[row.ID] = row
	s.versions[row.ID] = []model.ComposeStrategyVersion{{StrategyID: row.ID, Version: 1, Snapshot: row, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt}}
	return row, nil
}
func (s *strategyMemoryStore) UpdateStrategy(row model.ComposeStrategy, expected uint32) (model.ComposeStrategy, bool, error) {
	old := s.rows[row.ID]
	if old.CurrentVersion != expected {
		return model.ComposeStrategy{}, false, nil
	}
	row.CreatedAt, row.CreatedBy = old.CreatedAt, old.CreatedBy
	row.CurrentVersion = expected + 1
	s.rows[row.ID] = row
	s.versions[row.ID] = append([]model.ComposeStrategyVersion{{StrategyID: row.ID, Version: row.CurrentVersion, Snapshot: row, CreatedBy: row.UpdatedBy, CreatedAt: row.UpdatedAt}}, s.versions[row.ID]...)
	s.updates++
	return row, true, nil
}
func (s *strategyMemoryStore) FindStrategy(id int64) (model.ComposeStrategy, bool, error) {
	row, ok := s.rows[id]
	return row, ok, nil
}
func (s *strategyMemoryStore) ListStrategies(_ repository.StrategyFilter) ([]model.ComposeStrategy, error) {
	out := make([]model.ComposeStrategy, 0, len(s.rows))
	for _, row := range s.rows {
		out = append(out, row)
	}
	return out, nil
}
func (s *strategyMemoryStore) ListStrategyVersions(id int64) ([]model.ComposeStrategyVersion, error) {
	return s.versions[id], nil
}

func senior(team identity.TeamID, games ...string) identityservice.PublicUser {
	return identityservice.PublicUser{ID: 7, Role: identityservice.RoleSeniorOperator, Status: identityservice.UserStatusEnabled, TeamID: &team, GameIDs: games}
}
func strategyInput() StrategyInput {
	return StrategyInput{TeamID: 2, GameID: "game-a", Name: "基础策略", ContentType: "highlight", TemplateKey: "basic_cut", TemplateVersion: "1.0", Parameters: model.StrategyParameters{VideoRatio: "16:9"}, AllowedOverrides: []string{"video_ratio"}, Status: "active"}
}
func TestStrategyCreateUpdateCopyRetainsVersions(t *testing.T) {
	store := &strategyMemoryStore{}
	svc := NewStrategyService(store, func(id string) (bool, error) { return id == "game-a", nil })
	svc.now = func() time.Time { return time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC) }
	actor := senior(2, "game-a")
	created, err := svc.Create(actor, strategyInput())
	if err != nil || created.CurrentVersion != 1 {
		t.Fatalf("create = %+v, %v", created, err)
	}
	input := strategyInput()
	input.Parameters.VideoRatio = "9:16"
	input.Name = "竖屏策略"
	updated, err := svc.Update(actor, created.ID, 1, input)
	if err != nil || updated.CurrentVersion != 2 || updated.Parameters.VideoRatio != "9:16" {
		t.Fatalf("update = %+v, %v", updated, err)
	}
	if _, err := svc.Update(actor, created.ID, 1, input); !errors.Is(err, ErrStrategyConflict) {
		t.Fatalf("stale update = %v", err)
	}
	versions, err := svc.Versions(actor, created.ID)
	if err != nil || len(versions) != 2 || versions[0].Snapshot.Parameters.VideoRatio != "9:16" || versions[1].Snapshot.Parameters.VideoRatio != "16:9" {
		t.Fatalf("versions = %+v, %v", versions, err)
	}
	copied, err := svc.Copy(actor, created.ID, "副本")
	if err != nil || copied.ID == created.ID || copied.CurrentVersion != 1 || copied.Name != "副本" {
		t.Fatalf("copy = %+v, %v", copied, err)
	}
}

func TestStrategyScopeAndInvalidParameters(t *testing.T) {
	store := &strategyMemoryStore{}
	svc := NewStrategyService(store, func(id string) (bool, error) { return id == "game-a", nil })
	owner := senior(2, "game-a")
	input := strategyInput()
	input.Parameters.BGM = true
	if _, err := svc.Create(owner, input); !errors.Is(err, ErrStrategyInvalid) {
		t.Fatalf("invalid params = %v", err)
	}
	created, err := svc.Create(owner, strategyInput())
	if err != nil {
		t.Fatal(err)
	}
	otherTeam := senior(3, "game-a")
	if _, err := svc.Get(otherTeam, created.ID); !errors.Is(err, ErrStrategyNotFound) {
		t.Fatalf("cross-team read = %v", err)
	}
	if _, err := svc.Update(otherTeam, created.ID, 1, strategyInput()); !errors.Is(err, ErrStrategyForbidden) {
		t.Fatalf("cross-team update = %v", err)
	}
	operator := identityservice.PublicUser{ID: 8, Role: identityservice.RoleOperator, Status: identityservice.UserStatusEnabled, TeamID: owner.TeamID, GameIDs: []string{"game-a"}}
	if _, err := svc.Get(operator, created.ID); err != nil {
		t.Fatalf("operator read = %v", err)
	}
	if _, err := svc.Update(operator, created.ID, 1, strategyInput()); !errors.Is(err, ErrStrategyForbidden) {
		t.Fatalf("operator update = %v", err)
	}
	if _, err := svc.Copy(operator, created.ID, "副本"); !errors.Is(err, ErrStrategyForbidden) {
		t.Fatalf("operator copy = %v", err)
	}
	if store.updates != 0 {
		t.Fatalf("unauthorized updates: %d", store.updates)
	}
}
