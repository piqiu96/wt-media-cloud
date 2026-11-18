package service

import (
	"github.com/wt-media/wt-media-cloud/internal/config"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"time"
)

type RecoveryRepository interface {
	Get(uint64) (model.Task, error)
	ResolveFailure(model.Task, int64, model.FailureResolution, time.Time) (model.Task, error)
	Retry(model.Task, uint64, int64, time.Time) (model.Task, error)
	OriginalAttempt(uint64, uint64, int64) (model.OriginalAttemptState, error)
}
type RecoveryService struct {
	Repo    RecoveryRepository
	Enabled bool
}

func WiredRecovery() *RecoveryService {
	return &RecoveryService{Repo: ExecutionStore(), Enabled: config.Get().App.PublishingBaiduExecution}
}
func (s *RecoveryService) readable(actor identity.PublicUser, id uint64) (model.Task, error) {
	v, e := s.Repo.Get(id)
	if e == nil && !CanRead(actor, v) {
		return model.Task{}, ErrNotFound
	}
	return v, e
}
func (s *RecoveryService) Resolve(actor identity.PublicUser, id uint64, in model.FailureResolution) (model.Task, error) {
	v, e := s.readable(actor, id)
	if e != nil {
		return v, e
	}
	if !in.Valid() {
		return model.Task{}, ErrInvalid
	}
	return s.Repo.ResolveFailure(v, int64(actor.ID), in, time.Now().UTC())
}
func (s *RecoveryService) Retry(actor identity.PublicUser, id, revision uint64) (model.Task, error) {
	v, e := s.readable(actor, id)
	if e != nil {
		return v, e
	}
	if !s.Enabled || v.Platform != "baidu" {
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
