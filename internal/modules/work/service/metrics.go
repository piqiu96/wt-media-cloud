package service

import (
	"encoding/hex"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/repository"
	"time"
)

type MetricsRepository interface {
	RequestMetrics(uint64, time.Time) (model.Metrics, error)
	ClaimMetrics(identity.PublicUser, time.Time) (*model.MetricsClaim, error)
	ReportMetrics(int64, uint64, model.MetricsReport, time.Time) (model.Metrics, error)
}
type MetricsService struct {
	Works *Service
	Repo  MetricsRepository
}

func WiredMetrics() *MetricsService {
	return &MetricsService{Works: Wired(), Repo: repository.Store{DB: database.DB()}}
}
func (s *MetricsService) Request(a identity.PublicUser, id uint64, now time.Time) (model.Metrics, error) {
	w, err := s.Works.Get(a, id)
	if err != nil {
		return model.Metrics{}, err
	}
	if w.Platform != "bilibili" {
		return model.Metrics{}, ErrInvalid
	}
	return s.Repo.RequestMetrics(id, now)
}
func (s *MetricsService) Claim(a identity.PublicUser, now time.Time) (*model.MetricsClaim, error) {
	if err := validate(a, Filter{Page: 1, PageSize: 1}); err != nil {
		return nil, err
	}
	return s.Repo.ClaimMetrics(a, now)
}
func (s *MetricsService) Report(user int64, id uint64, in model.MetricsReport, now time.Time) (model.Metrics, error) {
	key, e := hex.DecodeString(in.LeaseToken)
	validError := in.Error == "unavailable" || in.Error == "rate_limited" || in.Error == "identity_mismatch" || in.Error == "invalid_response"
	if user <= 0 || id == 0 || e != nil || len(key) != 32 || in.PlatformWorkID == "" || len(in.PlatformWorkID) > 255 || in.PlatformAccountUID == "" || len(in.PlatformAccountUID) > 255 || (in.Error == "" && (in.Counts == nil || !in.Counts.Valid())) || (in.Error != "" && (!validError || in.Counts != nil)) {
		return model.Metrics{}, ErrInvalid
	}
	return s.Repo.ReportMetrics(user, id, in, now)
}
