package service

import (
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"testing"
	"time"
)

type recoveryFake struct {
	RecoveryRepository
	task          model.Task
	writes, reads int
}

func (f *recoveryFake) Get(uint64) (model.Task, error) { return f.task, nil }
func (f *recoveryFake) ResolveFailure(v model.Task, a int64, in model.FailureResolution, n time.Time) (model.Task, error) {
	f.writes++
	return v, nil
}
func (f *recoveryFake) Retry(v model.Task, r uint64, a int64, n time.Time) (model.Task, error) {
	f.writes++
	return v, nil
}
func (f *recoveryFake) OriginalAttempt(task, attempt uint64, user int64) (model.OriginalAttemptState, error) {
	f.reads++
	return model.OriginalAttemptState{AttemptID: attempt}, nil
}
func TestRecoveryRequiresCurrentScopeAndNeverUsesAnOldResultAsAuthorization(t *testing.T) {
	f := &recoveryFake{task: model.Task{ID: 1, OperatorID: 3, CategoryID: 4, TeamID: 1, Platform: "baidu"}}
	s := RecoveryService{Repo: f}
	owner := identity.PublicUser{ID: 3, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: []identity.CategoryID{4}}
	in := model.FailureResolution{AttemptID: 2, ExpectedRevision: 3, Conclusion: "uncertain", Evidence: model.VerificationEvidence{PlatformUID: "original", Reason: "等待人工核对", Basis: "平台安全验证尚未完成，保留原尝试待核实"}}
	for _, actor := range []identity.PublicUser{{ID: 99, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: owner.CategoryIDs}, {ID: 3, Role: identity.RoleOperator, Status: identity.UserStatusEnabled}, {ID: 3, Role: identity.RoleOperator, Status: identity.UserStatusDisabled, CategoryIDs: owner.CategoryIDs}} {
		if _, e := s.Resolve(actor, 1, in); !errors.Is(e, ErrNotFound) {
			t.Fatal(e)
		}
		if _, e := s.Retry(actor, 1, 3); !errors.Is(e, ErrNotFound) {
			t.Fatal(e)
		}
	}
	if f.writes != 0 {
		t.Fatal("foreign or disabled user reached write")
	}
	bad := in
	bad.Evidence.Basis = ""
	if _, e := s.Resolve(owner, 1, bad); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := s.Resolve(owner, 1, in); e != nil || f.writes != 1 {
		t.Fatal(e)
	}
	f.task.Platform = "bilibili"
	if _, e := s.Retry(owner, 1, 3); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
	f.task.Platform = "baidu"
	if _, e := s.Retry(owner, 1, 0); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := s.Retry(owner, 1, 3); e != nil || f.writes != 2 {
		t.Fatal(e)
	}
	for _, ids := range [][3]uint64{{0, 1, 2}, {3, 0, 2}, {3, 1, 0}} {
		if _, e := s.OriginalAttempt(int64(ids[0]), ids[1], ids[2]); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	if _, e := s.OriginalAttempt(3, 1, 2); e != nil || f.reads != 1 || f.writes != 2 {
		t.Fatal("original read changed authorization")
	}
}

func (f *recoveryFake) Cancel(v model.Task, in model.Cancellation, actor int64, now time.Time) (model.Task, error) {
	f.writes++
	return v, nil
}
func (f *recoveryFake) Abandon(v model.Task, actor int64, in model.Abandonment, now time.Time) (model.Task, error) {
	f.writes++
	return v, nil
}
func TestTerminationRechecksCurrentPermissionAndInput(t *testing.T) {
	f := &recoveryFake{task: model.Task{ID: 1, OperatorID: 3, CategoryID: 4, TeamID: 1, Platform: "baidu"}}
	s := RecoveryService{Repo: f}
	owner := identity.PublicUser{ID: 3, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: []identity.CategoryID{4}}
	in := model.Abandonment{AttemptID: 2, ExpectedRevision: 3, Evidence: model.VerificationEvidence{PlatformUID: "original", Reason: "原资源无法恢复", Basis: "已检查原账号和旧进程，没有可继续发布稿件，原现场已结束", SceneClosed: true, NoPublishableWork: true}}
	for _, actor := range []identity.PublicUser{{ID: 99, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, CategoryIDs: owner.CategoryIDs}, {ID: 3, Role: identity.RoleOperator, Status: identity.UserStatusDisabled, CategoryIDs: owner.CategoryIDs}, {ID: 3, Role: identity.RoleOperator, Status: identity.UserStatusEnabled}} {
		if _, err := s.Cancel(actor, 1, model.Cancellation{ExpectedRevision: 3, Reason: "停止当前安排"}); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, err := s.Abandon(actor, 1, in); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	if f.writes != 0 {
		t.Fatal("unauthorized mutation")
	}
	if _, err := s.Cancel(owner, 1, model.Cancellation{Reason: "停止当前安排"}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	invalid := in
	invalid.Evidence.SceneClosed = false
	if _, err := s.Abandon(owner, 1, invalid); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Cancel(owner, 1, model.Cancellation{ExpectedRevision: 3, Reason: "停止当前安排"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Abandon(owner, 1, in); err != nil {
		t.Fatal(err)
	}
	if f.writes != 2 {
		t.Fatalf("writes=%d", f.writes)
	}
}
