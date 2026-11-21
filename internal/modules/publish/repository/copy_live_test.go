package repository_test

import (
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	service "github.com/wt-media/wt-media-cloud/internal/modules/publish/service"
	"testing"
	"time"
)

func TestLiveCopyChecksExactSafeAttemptWithoutCreatingWork(t *testing.T) {
	s, c, original := unresolvedPublication(t)
	actor := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: []identity.CategoryID{1}}
	reader := service.RecoveryService{Repo: s}
	v, _ := s.Get(c.Task.ID)
	if _, err := reader.Copy(actor, v.ID, v.Revision); err == nil {
		t.Fatal("unknown copied")
	}
	v, err := s.ResolveFailure(v, 9, resolution(c, v), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	input, err := reader.Copy(actor, v.ID, v.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if input.Content.Description != v.Content.Description {
		t.Fatal("lost original text")
	}
	current, _ := s.Get(v.ID)
	history, _ := s.Attempt(v.ID)
	if current.Status != model.StatusFailed || current.Revision != v.Revision || len(history) != 1 || *history[0].ResultHash != original.ResultHash {
		t.Fatal("copy changed source")
	}
	var tasks int64
	if err := s.DB.Model(&model.Task{}).Count(&tasks).Error; err != nil || tasks != 1 {
		t.Fatalf("copy created task: %d %v", tasks, err)
	}
}
