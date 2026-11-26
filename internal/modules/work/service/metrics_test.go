package service

import (
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	"strings"
	"testing"
	"time"
)

type fakeMetrics struct{ calls int }

func (f *fakeMetrics) RequestMetrics(uint64, time.Time) (model.Metrics, error) {
	f.calls++
	return model.Metrics{}, nil
}
func (f *fakeMetrics) ClaimMetrics(identity.PublicUser, time.Time) (*model.MetricsClaim, error) {
	f.calls++
	return nil, nil
}
func (f *fakeMetrics) ReportMetrics(int64, uint64, model.MetricsReport, time.Time) (model.Metrics, error) {
	f.calls++
	return model.Metrics{}, nil
}
func TestMetricsValidationAndScope(t *testing.T) {
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	f := &fakeMetrics{}
	works := &fakeWork{v: model.Work{ID: 1, Platform: "bilibili", OperatorID: 9, TeamID: 7, CategoryID: 1}}
	s := MetricsService{Works: &Service{Repo: works}, Repo: f}
	now := time.Now()
	if _, e := s.Request(a, 1, now); e != nil {
		t.Fatal(e)
	}
	a.ID = 10
	if _, e := s.Request(a, 1, now); !errors.Is(e, ErrNotFound) || f.calls != 1 {
		t.Fatal("foreign work refresh", e, f.calls)
	}
	a.ID = 9
	works.v.Platform = "baidu"
	if _, e := s.Request(a, 1, now); !errors.Is(e, ErrInvalid) || f.calls != 1 {
		t.Fatal("unsupported refresh", e)
	}
	zero := uint64(0)
	in := model.MetricsReport{LeaseToken: strings.Repeat("a", 64), PlatformWorkID: "123", PlatformAccountUID: "456", Counts: &model.Counts{View: &zero, Like: &zero, Favorite: &zero, Coin: &zero, Reply: &zero, Danmaku: &zero}}
	if _, e := s.Report(9, 1, in, now); e != nil {
		t.Fatal("real zero rejected", e)
	}
	for _, change := range []func(*model.MetricsReport){
		func(v *model.MetricsReport) { v.Counts = nil },
		func(v *model.MetricsReport) { v.Counts = &model.Counts{View: &zero} },
		func(v *model.MetricsReport) { v.Error = "unavailable" },
		func(v *model.MetricsReport) { v.Error = "raw platform secret"; v.Counts = nil },
		func(v *model.MetricsReport) { v.LeaseToken = "bad" },
	} {
		v := in
		change(&v)
		before := f.calls
		if _, e := s.Report(9, 1, v, now); !errors.Is(e, ErrInvalid) || f.calls != before {
			t.Fatal("bad packet reached repository", e)
		}
	}
	in.Counts = nil
	in.Error = "rate_limited"
	if _, e := s.Report(9, 1, in, now); e != nil {
		t.Fatal(e)
	}
	unsafe := uint64(9007199254740992)
	in.Error = ""
	in.Counts = &model.Counts{View: &unsafe, Like: &zero, Favorite: &zero, Coin: &zero, Reply: &zero, Danmaku: &zero}
	if _, e := s.Report(9, 1, in, now); !errors.Is(e, ErrInvalid) {
		t.Fatal("unsafe JS number accepted")
	}
}
