package service

import (
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/work/model"
	"testing"
	"time"
)

type fakeWork struct {
	v            model.Work
	filter       Filter
	from, before time.Time
	reads        int
}

func (f *fakeWork) Get(uint64) (model.Work, error) { f.reads++; return f.v, nil }
func (f *fakeWork) List(in Filter) ([]model.Work, int64, error) {
	f.reads++
	f.filter = in
	return []model.Work{f.v}, 1, nil
}
func (f *fakeWork) Summary(in Filter, from, before time.Time) (Summary, error) {
	f.reads++
	f.filter = in
	f.from = from
	f.before = before
	return Summary{All: 1, Today: 1}, nil
}
func TestWorkReadScopeAndActualBusinessDay(t *testing.T) {
	team := identity.TeamID(7)
	a := identity.PublicUser{ID: 9, Role: identity.RoleOperator, Status: identity.UserStatusEnabled, TeamID: &team, CategoryIDs: []identity.CategoryID{1}}
	f := &fakeWork{v: model.Work{ID: 1, OperatorID: 9, TeamID: 7, CategoryID: 1}}
	s := Service{Repo: f}
	if _, err := s.Get(a, 1); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*identity.PublicUser){func(x *identity.PublicUser) { x.ID = 10 }, func(x *identity.PublicUser) { x.CategoryIDs = nil }, func(x *identity.PublicUser) { x.Status = identity.UserStatusDisabled }} {
		bad := a
		change(&bad)
		if _, err := s.Get(bad, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign scope: %v", err)
		}
	}
	in := Filter{Page: 1, PageSize: 20, MediaAccountID: 12, CategoryID: 1, Search: "标题"}
	now := time.Date(2026, 11, 20, 16, 0, 0, 0, time.UTC)
	if _, err := s.Summary(a, in, now); err != nil {
		t.Fatal(err)
	}
	if !f.from.Equal(now) || !f.before.Equal(now.Add(24*time.Hour)) || f.filter.Actor.ID != a.ID || f.filter.MediaAccountID != 12 || f.filter.Search != in.Search {
		t.Fatalf("day or scope changed: %+v %+v %v %v", in, f.filter, f.from, f.before)
	}
	now = now.Add(-time.Nanosecond)
	s.Summary(a, in, now)
	if !f.from.Equal(time.Date(2026, 11, 19, 16, 0, 0, 0, time.UTC)) {
		t.Fatal("midnight used server or observation day")
	}
	before := f.reads
	bad := in
	bad.PageSize = 101
	if _, _, err := s.List(a, bad); !errors.Is(err, ErrInvalid) || f.reads != before {
		t.Fatal("invalid query reached repository")
	}
}
