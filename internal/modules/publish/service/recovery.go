package service

import (
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"time"
)

type RecoveryRepository interface {
	GetHumanScene(model.Task, int64, model.HumanContinuation) (model.ExecutionFeedback, error)
	Resume(model.Task, int64, model.HumanContinuation, time.Time) (model.ExecutionFeedback, error)
	Get(uint64) (model.Task, error)
	Cancel(model.Task, model.Cancellation, int64, time.Time) (model.Task, error)
	Abandon(model.Task, int64, model.Abandonment, time.Time) (model.Task, error)
	ResolveResult(model.Task, int64, model.ResultResolution, time.Time) (model.Task, error)
	Retry(model.Task, uint64, int64, time.Time) (model.Task, error)
	OriginalAttempt(uint64, uint64, int64) (model.OriginalAttemptState, error)
}
type RecoveryService struct {
	Repo RecoveryRepository
}

func WiredRecovery() *RecoveryService {
	return &RecoveryService{Repo: ExecutionStore()}
}
func (s *RecoveryService) readable(actor identity.PublicUser, id uint64) (model.Task, error) {
	v, e := s.Repo.Get(id)
	if e == nil && !CanRead(actor, v) {
		return model.Task{}, ErrNotFound
	}
	return v, e
}
func (s *RecoveryService) Resolve(actor identity.PublicUser, id uint64, in model.ResultResolution) (model.Task, error) {
	v, e := s.readable(actor, id)
	if e != nil {
		return v, e
	}
	if !in.Valid() {
		return model.Task{}, ErrInvalid
	}
	if v.Platform != "baidu" {
		return model.Task{}, ErrClosed
	}
	return s.Repo.ResolveResult(v, int64(actor.ID), in, time.Now().UTC())
}
func (s *RecoveryService) Retry(actor identity.PublicUser, id, revision uint64) (model.Task, error) {
	if !canExecutePublication(actor) {
		return model.Task{}, ErrNotFound
	}
	v, e := s.readable(actor, id)
	if e != nil {
		return v, e
	}
	if v.Platform != "baidu" {
		return model.Task{}, ErrClosed
	}
	if revision == 0 {
		return model.Task{}, ErrInvalid
	}
	return s.Repo.Retry(v, revision, int64(actor.ID), time.Now().UTC())
}
func (s *RecoveryService) OriginalAttempt(user int64, id, attempt uint64) (model.OriginalAttemptState, error) {
	if user <= 0 || id == 0 || attempt == 0 {
		return model.OriginalAttemptState{}, ErrInvalid
	}
	return s.Repo.OriginalAttempt(id, attempt, user)
}

func (s *RecoveryService) Cancel(actor identity.PublicUser, id uint64, in model.Cancellation) (model.Task, error) {
	v, err := s.readable(actor, id)
	if err != nil {
		return v, err
	}
	if !in.Valid() {
		return model.Task{}, ErrInvalid
	}
	return s.Repo.Cancel(v, in, int64(actor.ID), time.Now().UTC())
}
func (s *RecoveryService) Abandon(actor identity.PublicUser, id uint64, in model.Abandonment) (model.Task, error) {
	v, err := s.readable(actor, id)
	if err != nil {
		return v, err
	}
	if !in.Valid() {
		return model.Task{}, ErrInvalid
	}
	return s.Repo.Abandon(v, int64(actor.ID), in, time.Now().UTC())
}

func (s *RecoveryService) GetHumanScene(user int64, id uint64, in model.HumanContinuation) (model.ExecutionFeedback, error) {
	if user <= 0 || id == 0 || !in.Valid() {
		return model.ExecutionFeedback{}, ErrInvalid
	}
	v, err := s.Repo.Get(id)
	if err != nil {
		return model.ExecutionFeedback{}, err
	}
	if v.OperatorID != user {
		return model.ExecutionFeedback{}, ErrNotFound
	}
	return s.Repo.GetHumanScene(v, user, in)
}
func (s *RecoveryService) Resume(user int64, id uint64, in model.HumanContinuation) (model.ExecutionFeedback, error) {
	if user <= 0 || id == 0 || !in.Valid() {
		return model.ExecutionFeedback{}, ErrInvalid
	}
	v, err := s.Repo.Get(id)
	if err != nil {
		return model.ExecutionFeedback{}, err
	}
	if v.OperatorID != user {
		return model.ExecutionFeedback{}, ErrNotFound
	}
	return s.Repo.Resume(v, user, in, time.Now().UTC())
}
