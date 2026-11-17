package service

import (
	"context"
	"encoding/json"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publishing/model"
	"testing"
	"time"
)

type executionFake struct {
	ExecutionRepository
	v                                   model.Task
	claim                               *model.Claim
	starts, claims, grants, completions int
}

func (f *executionFake) Get(uint64) (model.Task, error) { return f.v, nil }
func (f *executionFake) Start(v model.Task, r uint64, a int64, n time.Time) (model.Task, error) {
	f.starts++
	return v, nil
}
func (f *executionFake) Claim(u int64, k string, n time.Time) (*model.Claim, error) {
	f.claims++
	return f.claim, nil
}
func (f *executionFake) Complete(v model.Task, u int64, c model.Completion, n time.Time) (model.ExecutionFeedback, error) {
	f.completions++
	return model.ExecutionFeedback{}, nil
}
func TestExecutionRequiresActorScopeAndExplicitOpening(t *testing.T) {
	f := &executionFake{v: model.Task{ID: 1, OperatorID: 3, Platform: "baidu"}}
	s := &ExecutionService{Repo: f}
	a := identity.PublicUser{ID: 3, Role: identity.RoleAdmin, Status: identity.UserStatusEnabled}
	if _, e := s.Start(a, 1, 1); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
	if _, e := s.Claim(context.Background(), 3, "safe-key"); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
	if f.starts+f.claims != 0 {
		t.Fatal("closed platform reached repository")
	}
	s.Enabled = true
	if _, e := s.Start(identity.PublicUser{ID: 99, Role: identity.RoleOperator, Status: identity.UserStatusEnabled}, 1, 1); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := s.Start(a, 1, 1); e != nil || f.starts != 1 {
		t.Fatal(e)
	}
}
func TestClaimOnlySignsRunnableOriginalAndNeverPersistsGrant(t *testing.T) {
	f := &executionFake{claim: &model.Claim{Task: model.Task{Status: "uncertain"}, Attempt: model.Attempt{Stage: "finished", Status: "interrupted"}, Replayed: true}}
	s := &ExecutionService{Repo: f, Enabled: true, Grant: func(context.Context, model.Claim) (FileGrants, error) {
		f.grants++
		return FileGrants{Video: FileGrant{URL: "https://private.test/?signature=never-save"}}, nil
	}}
	got, e := s.Claim(context.Background(), 3, "stable-key")
	if e != nil || got.Files != nil || f.grants != 0 {
		t.Fatalf("%+v %v", got, e)
	}
	f.claim.Task.Status = "running"
	f.claim.Attempt.Status = "running"
	f.claim.Attempt.Stage = "claimed"
	got, e = s.Claim(context.Background(), 3, "stable-key")
	if e != nil || got.Files == nil || f.grants != 1 {
		t.Fatalf("%+v %v", got, e)
	}
	raw, _ := json.Marshal(got.Attempt)
	if string(raw) == "" {
		t.Fatal("missing frozen input")
	}
	if _, e = s.Claim(context.Background(), 3, "bad key"); !errors.Is(e, ErrInvalid) || f.claims != 2 {
		t.Fatal(e)
	}
}
func TestResultPublicURLRejectsPrivateAddressesAndTokenQueries(t *testing.T) {
	for _, raw := range []string{"http://baijiahao.baidu.com/s?id=123", "https://user:secret@baijiahao.baidu.com/s?id=123", "https://baijiahao.baidu.com/s?id=1&token=x", "https://storage.test/file?signature=x", "https://baijiahao.baidu.com/s?id=1&id=2"} {
		if safePublicURL(raw) {
			t.Fatal(raw)
		}
	}
	if !safePublicURL("https://baijiahao.baidu.com/s?id=1834567890123456789") {
		t.Fatal("real public work address rejected")
	}
}
