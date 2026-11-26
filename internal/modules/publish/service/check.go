package service

import (
	"time"

	"github.com/wt-media/wt-media-cloud/internal/modules/publish/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/publish/repository"
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
	if user <= 0 || id == 0 {
		return model.PublicCheckAcknowledgement{}, ErrInvalid
	}
	v, err := s.Repo.Get(id)
	if err != nil {
		return model.PublicCheckAcknowledgement{}, err
	}
	if v.OperatorID != user {
		return model.PublicCheckAcknowledgement{}, ErrNotFound
	}
	if !in.ValidFor(v.Platform) {
		return model.PublicCheckAcknowledgement{}, ErrInvalid
	}
	return s.Repo.ObservePublication(v, user, in, time.Now().UTC())
}

func (s *PublicCheckService) ClaimV2(actor model.CheckActor, in model.CheckClaimInput) (any, error) {
	if _, ok := model.SupportedPublicationPlatforms(in.SupportedPlatforms); !ok {
		return nil, ErrInvalid
	}
	if actor.UserID <= 0 || !validKey(in.RequestKey) {
		return nil, ErrInvalid
	}
	repo, ok := s.Repo.(interface {
		ClaimPublicCheckV2(model.CheckActor, model.CheckClaimInput, time.Time) (any, error)
	})
	if !ok {
		return nil, repository.ErrAgentUpgrade
	}
	return repo.ClaimPublicCheckV2(actor, in, time.Now().UTC())
}
func (s *PublicCheckService) Release(actor model.CheckActor, id uint64, in model.PublicCheckRelease) (model.PublicCheckReleaseAck, error) {
	if actor.UserID <= 0 || id == 0 || !in.Valid() {
		return model.PublicCheckReleaseAck{}, ErrInvalid
	}
	repo, ok := s.Repo.(interface {
		ReleasePublicCheck(model.CheckActor, uint64, model.PublicCheckRelease, time.Time) (model.PublicCheckReleaseAck, error)
	})
	if !ok {
		return model.PublicCheckReleaseAck{}, repository.ErrAgentUpgrade
	}
	return repo.ReleasePublicCheck(actor, id, in, time.Now().UTC())
}
