package service

import (
	"context"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"testing"
)

func TestAdminCannotPrepareOrAuthorizePublication(t *testing.T) {
	admin := identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	t.Run("create", func(t *testing.T) {
		s, _, in := fixture()
		if _, err := s.Create(context.Background(), admin, in); !errors.Is(err, ErrNotFound) {
			t.Fatalf("admin created task: %v", err)
		}
		if len(s.Repo.(*memoryTasks).items) != 0 {
			t.Fatal("admin creation wrote task")
		}
	})
	t.Run("start", func(t *testing.T) {
		f := &executionFake{v: model.Task{ID: 7, OperatorID: 9, Platform: "baidu"}}
		if _, err := (&ExecutionService{Repo: f}).Start(admin, 7, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("admin started: %v", err)
		}
		if f.starts != 0 {
			t.Fatal("admin authorization reached repository")
		}
	})
	t.Run("retry", func(t *testing.T) {
		f := &recoveryFake{task: model.Task{ID: 7, OperatorID: 9, Platform: "baidu"}}
		if _, err := (&RecoveryService{Repo: f}).Retry(admin, 7, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("admin retried: %v", err)
		}
		if f.writes != 0 {
			t.Fatal("admin retry reached repository")
		}
	})
	t.Run("copy", func(t *testing.T) {
		f := &recoveryFake{task: model.Task{ID: 7, OperatorID: 9, Platform: "baidu", Revision: 1, Status: model.StatusPublished}}
		if _, err := (&RecoveryService{Repo: f}).Copy(admin, 7, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("admin copied: %v", err)
		}
	})
}

func TestAdminRetainsCloudPublicationReadAndEdit(t *testing.T) {
	s, operator, in := fixture()
	task, err := s.Create(context.Background(), operator, in)
	if err != nil {
		t.Fatal(err)
	}
	admin := identity.PublicUser{ID: 1, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	if _, err := s.Get(admin, task.ID); err != nil {
		t.Fatalf("admin cannot read: %v", err)
	}
	if rows, total, err := s.List(admin, Filter{Page: 1, PageSize: 10}); err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("admin list: %v %d %d", err, total, len(rows))
	}
	edited := in.Content
	edited.Description = "管理员修改云端待执行内容"
	if _, err := s.Edit(context.Background(), admin, task.ID, EditInput{ExpectedRevision: task.Revision, Content: edited}); err != nil {
		t.Fatalf("admin cannot manage content: %v", err)
	}
}
