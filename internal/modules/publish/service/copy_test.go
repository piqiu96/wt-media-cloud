package service

import (
	"encoding/json"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"strings"
	"testing"
)

type copyFake struct {
	recoveryFake
	original model.OriginalAttemptState
}

func (f *copyFake) OriginalAttempt(task, attempt uint64, user int64) (model.OriginalAttemptState, error) {
	f.reads++
	return f.original, nil
}
func TestCopyOnlyReturnsReusableContentFromAnAllowedOriginalTask(t *testing.T) {
	a := identity.PublicUser{ID: 3, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: []identity.CategoryID{4}}
	attempt := uint64(21)
	f := &copyFake{recoveryFake: recoveryFake{task: model.Task{ID: 7, Revision: 9, OperatorID: 3, CategoryID: 4, Platform: "baidu", CurrentAttemptID: &attempt, Content: model.Content{Description: "人工文案", Topics: "#话题", Declaration: "无需声明", Confirmed: true, CoverFileID: 99, CoverRender: &model.CoverRender{TemplateID: 4}}, RequestKey: "old-key"}}}
	s := RecoveryService{Repo: f}
	for _, status := range []model.Status{model.StatusPending, model.StatusQueued, model.StatusRunning, model.StatusAwaitingHuman, model.StatusUncertain, model.StatusReviewing, model.StatusFailed} {
		f.task.Status = status
		if _, err := s.Copy(a, 7, 9); !errors.Is(err, ErrConflict) {
			t.Fatalf("unsafe %s copied: %v", status, err)
		}
	}
	f.original = model.OriginalAttemptState{AttemptID: 21, SceneResolved: true, ResolvedStatus: model.StatusFailed}
	for _, status := range []model.Status{model.StatusFailed, model.StatusPublished, model.StatusCancelled, model.StatusAbandoned} {
		f.task.Status = status
		v, err := s.Copy(a, 7, 9)
		if err != nil {
			t.Fatal(status, err)
		}
		if v.Platform != "baidu" || v.CategoryID != 4 || v.Content.Description != "人工文案" || v.Content.Declaration != "无需声明" {
			t.Fatalf("lost content: %+v", v)
		}
		b, _ := json.Marshal(v)
		for _, forbidden := range []string{"cover_file_id", "cover_render", "confirmed", "request_key", "attempt", "account", "output", "authorized"} {
			if strings.Contains(string(b), forbidden) {
				t.Fatalf("unsafe copied field %s: %s", forbidden, b)
			}
		}
	}
	if _, err := s.Copy(a, 7, 8); !errors.Is(err, ErrConflict) {
		t.Fatal("stale copied", err)
	}
	foreign := a
	foreign.ID = 99
	if _, err := s.Copy(foreign, 7, 9); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign copied", err)
	}
	if f.writes != 0 {
		t.Fatal("copy mutated original")
	}
}
