package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
)

type PublicCheckRepository interface {
	Get(uint64) (model.Task, error)
	ClaimPublicCheck(int64, time.Time) (*model.PublicCheckClaim, error)
	ObservePublication(model.Task, int64, model.PublicCheckReport, time.Time) (model.PublicCheckAcknowledgement, error)
}
type PublicCheckService struct{ Repo PublicCheckRepository }

func WiredPublicCheck() *PublicCheckService { return &PublicCheckService{Repo: ExecutionStore()} }
func (s *PublicCheckService) Claim(user int64) (*model.PublicCheckClaim, error) {
	if user <= 0 {
		return nil, ErrInvalid
	}
	return s.Repo.ClaimPublicCheck(user, time.Now().UTC())
}
func (s *PublicCheckService) Observe(user int64, id uint64, in model.PublicCheckReport) (model.PublicCheckAcknowledgement, error) {
	if user <= 0 || id == 0 || !in.Valid() {
		return model.PublicCheckAcknowledgement{}, ErrInvalid
	}
	v, err := s.Repo.Get(id)
	if err != nil {
		return model.PublicCheckAcknowledgement{}, err
	}
	if v.OperatorID != user {
		return model.PublicCheckAcknowledgement{}, ErrNotFound
	}
	return s.Repo.ObservePublication(v, user, in, time.Now().UTC())
}
