package service

import (
	"errors"
	"testing"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

type checkFake struct {
	PublicCheckRepository
	task          model.Task
	reads, writes int
}

func (f *checkFake) Get(uint64) (model.Task, error) { f.reads++; return f.task, nil }
func (f *checkFake) ClaimPublicCheck(user int64, now time.Time) (*model.PublicCheckClaim, error) {
	f.writes++
	return nil, nil
}
func (f *checkFake) ObservePublication(v model.Task, user int64, in model.PublicCheckReport, now time.Time) (model.PublicCheckAcknowledgement, error) {
	f.writes++
	return model.PublicCheckAcknowledgement{}, nil
}
func TestPublicCheckRequiresOriginalOwnerAndRejectsFailureConclusion(t *testing.T) {
	f := &checkFake{task: model.Task{ID: 1, Platform: "baidu", OperatorID: 9}}
	s := PublicCheckService{Repo: f}
	in := model.PublicCheckReport{Generation: 1, Observation: model.ResultResolution{AttemptID: 2, ExpectedRevision: 3, Conclusion: model.StatusUncertain, Evidence: model.VerificationEvidence{PlatformUID: "original", Reason: "暂时不可访问", Basis: "指定稿件尚未核实匿名公开，保留审核中"}}}
	if _, err := s.Claim(0); !errors.Is(err, ErrInvalid) {
		t.Fatal("missing identity admitted")
	}
	if _, err := s.Observe(10, 1, in); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign observation admitted")
	}
	if f.writes != 0 {
		t.Fatal("unauthorized mutation")
	}
	in.Observation.Conclusion = model.StatusFailed
	in.Observation.Evidence.SceneClosed = true
	in.Observation.Evidence.NoPublishableWork = true
	if _, err := s.Observe(9, 1, in); !errors.Is(err, ErrInvalid) {
		t.Fatal("automatic failure admitted")
	}
	if f.writes != 0 {
		t.Fatal("automatic failure mutated task")
	}
	in.Observation.Conclusion = model.StatusUncertain
	if _, err := s.Observe(9, 1, in); err != nil || f.writes != 1 {
		t.Fatal(err)
	}
}
