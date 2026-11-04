package service

import (
	"context"
	"errors"
	identity "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"math"
	"testing"
)

type strategyPageFake struct {
	strategyMemoryStore
	calls      int
	page, size int
}

func (f *strategyPageFake) Page(_ context.Context, filter repository.StrategyFilter, page, size int) (repository.ComposeStrategyPage, error) {
	f.calls++
	f.lastFilter = filter
	f.page = page
	f.size = size
	return repository.ComposeStrategyPage{Items: f.items, Total: 205, Page: page, PageSize: size}, nil
}
func TestStrategyPagePreservesOrdinaryOwnerScopeAndNormalizesPagination(t *testing.T) {
	base, _, actor := strategyFixture()
	actor.Role = identity.RoleOperator
	fake := &strategyPageFake{}
	base.store = fake
	page, err := base.Page(context.Background(), actor, ComposeStrategyListQuery{})
	if err != nil || page.Page != 1 || page.PageSize != 20 || page.Total != 205 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	f := fake.lastFilter
	if f.TeamID != 7 || f.OperatorID != 9 || !f.ActiveOnly || len(f.CategoryIDs) != 1 || f.CategoryIDs[0] != 1 {
		t.Fatalf("scope=%+v", f)
	}
}
func TestStrategyPageRejectsScopeExpansionAndBadPaginationBeforeSQL(t *testing.T) {
	for _, tc := range []struct {
		query ComposeStrategyListQuery
		want  error
	}{{ComposeStrategyListQuery{TeamID: 8}, ErrStrategyForbidden}, {ComposeStrategyListQuery{OperatorID: 10}, ErrStrategyForbidden}, {ComposeStrategyListQuery{CategoryID: 4}, ErrStrategyForbidden}, {ComposeStrategyListQuery{Page: -1}, ErrStrategyInvalid}, {ComposeStrategyListQuery{Page: math.MaxInt, PageSize: 100}, ErrStrategyInvalid}, {ComposeStrategyListQuery{PageSize: 101}, ErrStrategyInvalid}, {ComposeStrategyListQuery{TeamID: -1}, ErrStrategyInvalid}, {ComposeStrategyListQuery{TemplateID: -1}, ErrStrategyInvalid}, {ComposeStrategyListQuery{Status: "future"}, ErrStrategyInvalid}} {
		base, _, actor := strategyFixture()
		actor.Role = identity.RoleOperator
		fake := &strategyPageFake{}
		base.store = fake
		_, err := base.Page(context.Background(), actor, tc.query)
		if !errors.Is(err, tc.want) || fake.calls != 0 {
			t.Fatalf("query=%+v err=%v calls=%d", tc.query, err, fake.calls)
		}
	}
}
func TestStrategyPageEmptyCategoriesDoNotReachRepository(t *testing.T) {
	base, _, actor := strategyFixture()
	actor.Role = identity.RoleOperator
	actor.CategoryIDs = nil
	fake := &strategyPageFake{}
	base.store = fake
	page, err := base.Page(context.Background(), actor, ComposeStrategyListQuery{Page: 3, PageSize: 10})
	if err != nil || page.Total != 0 || len(page.Items) != 0 || fake.calls != 0 || page.Page != 3 {
		t.Fatalf("page=%+v err=%v calls=%d", page, err, fake.calls)
	}
}
func TestStrategyPageManagementNarrowingDoesNotBroadenTeam(t *testing.T) {
	base, _, actor := strategyFixture()
	fake := &strategyPageFake{}
	base.store = fake
	_, err := base.Page(context.Background(), actor, ComposeStrategyListQuery{OperatorID: 10, CategoryID: 4, Page: 2, PageSize: 50})
	if err != nil || fake.lastFilter.TeamID != 7 || fake.lastFilter.OperatorID != 10 || len(fake.lastFilter.CategoryIDs) != 0 || fake.page != 2 || fake.size != 50 {
		t.Fatalf("filter=%+v err=%v", fake.lastFilter, err)
	}
	_, err = base.Page(context.Background(), actor, ComposeStrategyListQuery{TeamID: 8})
	if !errors.Is(err, ErrStrategyForbidden) || fake.calls != 1 {
		t.Fatalf("foreign team err=%v calls=%d", err, fake.calls)
	}
	actor.Role = identity.RoleAdmin
	actor.TeamID = nil
	_, err = base.Page(context.Background(), actor, ComposeStrategyListQuery{TeamID: 8, OperatorID: 10})
	if err != nil || fake.lastFilter.TeamID != 8 || fake.lastFilter.OperatorID != 10 {
		t.Fatalf("admin filter=%+v err=%v", fake.lastFilter, err)
	}
}

func TestStrategyPageDisabledOrTeamlessActorCannotQuery(t *testing.T) {
	for _, mutate := range []func(*identity.PublicUser){func(actor *identity.PublicUser) { actor.Status = "disabled" }, func(actor *identity.PublicUser) { actor.TeamID = nil }, func(actor *identity.PublicUser) { actor.Role = "unknown" }} {
		base, _, actor := strategyFixture()
		actor.Role = identity.RoleOperator
		mutate(&actor)
		fake := &strategyPageFake{}
		base.store = fake
		_, err := base.Page(context.Background(), actor, ComposeStrategyListQuery{})
		if !errors.Is(err, ErrStrategyForbidden) || fake.calls != 0 {
			t.Fatalf("actor=%+v err=%v calls=%d", actor, err, fake.calls)
		}
	}
}
